package workload

import (
	"context"
	"fmt"
	_ "strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	_ "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/celikgo/autoz-control-tower/internal/cluster"
)

// Manager handles workload operations across multiple clusters
// This is like a "universal remote control" for your Kubernetes workloads
type Manager struct {
	clusterManager *cluster.Manager
}

// NewManager creates a new workload manager
func NewManager(clusterManager *cluster.Manager) *Manager {
	return &Manager{
		clusterManager: clusterManager,
	}
}

// DeploymentInfo contains information about a deployment across clusters
type DeploymentInfo struct {
	ClusterName   string `json:"clusterName"`
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	Replicas      int32  `json:"replicas"`
	ReadyReplicas int32  `json:"readyReplicas"`
	Image         string `json:"image"`
	Status        string `json:"status"`
	Age           string `json:"age"`
	Error         string `json:"error,omitempty"`
}

// PodInfo contains information about pods across clusters
type PodInfo struct {
	ClusterName string    `json:"clusterName"`
	Namespace   string    `json:"namespace"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Ready       string    `json:"ready"`
	Restarts    int32     `json:"restarts"`
	Age         string    `json:"age"`
	Node        string    `json:"node"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ListDeployments retrieves deployments from specified clusters
// This is like asking "show me all my applications" across multiple data centers
func (m *Manager) ListDeployments(clusterNames []string, namespace string) ([]DeploymentInfo, error) {
	// If no clusters specified, use all available clusters
	if len(clusterNames) == 0 {
		for _, status := range m.clusterManager.ListClusters() {
			if status.Connected {
				clusterNames = append(clusterNames, status.Name)
			}
		}
	}

	// Use channels to collect results from multiple clusters in parallel
	resultChan := make(chan []DeploymentInfo, len(clusterNames))
	var wg sync.WaitGroup

	// Query each cluster in parallel for better performance
	for _, clusterName := range clusterNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			deployments := m.getDeploymentsFromCluster(name, namespace)
			resultChan <- deployments
		}(clusterName)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect all results
	var allDeployments []DeploymentInfo
	for deployments := range resultChan {
		allDeployments = append(allDeployments, deployments...)
	}

	return allDeployments, nil
}

// getDeploymentsFromCluster retrieves deployments from a single cluster
// This handles the actual Kubernetes API interaction for one cluster
func (m *Manager) getDeploymentsFromCluster(clusterName, namespace string) []DeploymentInfo {
	client, err := m.clusterManager.GetClient(clusterName)
	if err != nil {
		return []DeploymentInfo{{
			ClusterName: clusterName,
			Error:       fmt.Sprintf("Failed to get cluster client: %v", err),
		}}
	}

	// Use a timeout to prevent hanging on slow clusters
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get deployments from the Kubernetes API
	deployments, err := client.Clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return []DeploymentInfo{{
			ClusterName: clusterName,
			Error:       fmt.Sprintf("Failed to list deployments: %v", err),
		}}
	}

	var result []DeploymentInfo
	for _, deployment := range deployments.Items {
		// Extract the main container image (usually the first container)
		image := "unknown"
		if len(deployment.Spec.Template.Spec.Containers) > 0 {
			image = deployment.Spec.Template.Spec.Containers[0].Image
		}

		// Determine deployment status based on replica counts
		// We explicitly handle all cases to make the logic clear and maintainable
		var status string
		if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
			status = "Ready"
		} else if deployment.Status.ReadyReplicas > 0 {
			status = "Partial"
		} else if deployment.Status.ReadyReplicas == 0 {
			status = "NotReady"
		} else {
			// This case handles unexpected scenarios (e.g., negative replica counts)
			// which could indicate API issues or edge cases we haven't considered
			status = "Unknown"
		}

		// Calculate age of the deployment
		age := time.Since(deployment.CreationTimestamp.Time).Round(time.Second)

		result = append(result, DeploymentInfo{
			ClusterName:   clusterName,
			Namespace:     deployment.Namespace,
			Name:          deployment.Name,
			Replicas:      *deployment.Spec.Replicas,
			ReadyReplicas: deployment.Status.ReadyReplicas,
			Image:         image,
			Status:        status,
			Age:           formatDuration(age),
		})
	}

	return result
}

// ListPods retrieves pods from specified clusters with optional filtering
func (m *Manager) ListPods(clusterNames []string, namespace string, labelSelector string) ([]PodInfo, error) {
	if len(clusterNames) == 0 {
		for _, status := range m.clusterManager.ListClusters() {
			if status.Connected {
				clusterNames = append(clusterNames, status.Name)
			}
		}
	}

	resultChan := make(chan []PodInfo, len(clusterNames))
	var wg sync.WaitGroup

	for _, clusterName := range clusterNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			pods := m.getPodsFromCluster(name, namespace, labelSelector)
			resultChan <- pods
		}(clusterName)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var allPods []PodInfo
	for pods := range resultChan {
		allPods = append(allPods, pods...)
	}

	return allPods, nil
}

// getPodsFromCluster retrieves pods from a single cluster
func (m *Manager) getPodsFromCluster(clusterName, namespace, labelSelector string) []PodInfo {
	client, err := m.clusterManager.GetClient(clusterName)
	if err != nil {
		return []PodInfo{{
			ClusterName: clusterName,
			Name:        "error",
			Status:      fmt.Sprintf("Failed to get cluster client: %v", err),
		}}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	listOptions := metav1.ListOptions{}
	if labelSelector != "" {
		listOptions.LabelSelector = labelSelector
	}

	pods, err := client.Clientset.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		return []PodInfo{{
			ClusterName: clusterName,
			Name:        "error",
			Status:      fmt.Sprintf("Failed to list pods: %v", err),
		}}
	}

	var result []PodInfo
	for _, pod := range pods.Items {
		// Calculate ready containers
		readyContainers := 0
		totalContainers := len(pod.Spec.Containers)
		for _, condition := range pod.Status.ContainerStatuses {
			if condition.Ready {
				readyContainers++
			}
		}

		// Count total restarts
		var totalRestarts int32
		for _, containerStatus := range pod.Status.ContainerStatuses {
			totalRestarts += containerStatus.RestartCount
		}

		// Determine pod node
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			nodeName = "unscheduled"
		}

		result = append(result, PodInfo{
			ClusterName: clusterName,
			Namespace:   pod.Namespace,
			Name:        pod.Name,
			Status:      string(pod.Status.Phase),
			Ready:       fmt.Sprintf("%d/%d", readyContainers, totalContainers),
			Restarts:    totalRestarts,
			Age:         formatDuration(time.Since(pod.CreationTimestamp.Time)),
			Node:        nodeName,
			CreatedAt:   pod.CreationTimestamp.Time,
		})
	}

	return result
}

// DeployToCluster deploys a YAML manifest to a specific cluster
// This is like sending deployment instructions to a specific data center
func (m *Manager) DeployToCluster(clusterName, namespace, yamlContent string) error {
	client, err := m.clusterManager.GetClient(clusterName)
	if err != nil {
		return fmt.Errorf("failed to get cluster client for %s: %w", clusterName, err)
	}

	// Parse the YAML content to determine what type of resource we're deploying
	// This is a simplified parser - in production, you'd want more robust YAML handling
	var obj map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &obj); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	kind, ok := obj["kind"].(string)
	if !ok {
		return fmt.Errorf("YAML must specify a 'kind' field")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Handle different resource types - this example handles Deployments
	// In a full implementation, you'd want to handle many more resource types
	switch kind {
	case "Deployment":
		var deployment appsv1.Deployment
		if err := yaml.Unmarshal([]byte(yamlContent), &deployment); err != nil {
			return fmt.Errorf("failed to parse Deployment YAML: %w", err)
		}

		// Set namespace if not specified in YAML
		if deployment.Namespace == "" {
			deployment.Namespace = namespace
		}

		// Try to update if exists, create if not
		existing, err := client.Clientset.AppsV1().Deployments(deployment.Namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
		if err == nil {
			// Update existing deployment
			deployment.ResourceVersion = existing.ResourceVersion
			_, err = client.Clientset.AppsV1().Deployments(deployment.Namespace).Update(ctx, &deployment, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update deployment: %w", err)
			}
			fmt.Printf("Updated deployment %s in cluster %s\n", deployment.Name, clusterName)
		} else {
			// Create new deployment
			_, err = client.Clientset.AppsV1().Deployments(deployment.Namespace).Create(ctx, &deployment, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create deployment: %w", err)
			}
			fmt.Printf("Created deployment %s in cluster %s\n", deployment.Name, clusterName)
		}

	default:
		return fmt.Errorf("resource kind '%s' is not supported yet", kind)
	}

	return nil
}

// DeployToMultipleClusters deploys to multiple clusters in parallel
// This is like broadcasting deployment instructions to multiple data centers
func (m *Manager) DeployToMultipleClusters(clusterNames []string, namespace, yamlContent string) map[string]error {
	results := make(map[string]error)
	var wg sync.WaitGroup
	var mutex sync.Mutex

	for _, clusterName := range clusterNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			err := m.DeployToCluster(name, namespace, yamlContent)

			mutex.Lock()
			results[name] = err
			mutex.Unlock()
		}(clusterName)
	}

	wg.Wait()
	return results
}

// formatDuration converts a time.Duration to a human-readable string
// This mimics kubectl's duration formatting
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
