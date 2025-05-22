package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/celikgo/autoz-control-tower/internal/config"
)

// Manager handles connections to multiple Kubernetes clusters
// Think of this as your "cluster phone book" with active connections
type Manager struct {
	clients map[string]*ClusterClient // Map of cluster name to client
	config  *config.MultiClusterConfig
	mutex   sync.RWMutex // Protects concurrent access to the clients map
}

// ClusterClient wraps a Kubernetes client with cluster metadata
// This is like a "phone line" to a specific cluster
type ClusterClient struct {
	Config     config.ClusterConfig
	RestConfig *rest.Config
	Clientset  kubernetes.Interface // The actual Kubernetes client
	Connected  bool
	Error      error
}

// NewManager creates a new cluster manager and establishes connections
// This is like setting up your entire phone system at once
func NewManager(cfg *config.MultiClusterConfig) (*Manager, error) {
	manager := &Manager{
		clients: make(map[string]*ClusterClient),
		config:  cfg,
	}

	// Connect to all clusters in parallel for better performance
	// This is like dialing all your contacts simultaneously
	if err := manager.connectToAllClusters(); err != nil {
		return nil, fmt.Errorf("failed to connect to clusters: %w", err)
	}

	return manager, nil
}

// connectToAllClusters establishes connections to all configured clusters
// Uses goroutines for parallel connection - much faster than sequential
func (m *Manager) connectToAllClusters() error {
	var wg sync.WaitGroup
	connectionResults := make(chan *ClusterClient, len(m.config.Clusters))

	// Start connection attempts for all clusters in parallel
	for _, clusterConfig := range m.config.Clusters {
		wg.Add(1)
		go func(cc config.ClusterConfig) {
			defer wg.Done()
			client := m.connectToCluster(cc)
			connectionResults <- client
		}(clusterConfig)
	}

	// Wait for all connections to complete
	go func() {
		wg.Wait()
		close(connectionResults)
	}()

	// Collect results and check for any failures
	var connectionErrors []string
	successfulConnections := 0

	for client := range connectionResults {
		m.mutex.Lock()
		m.clients[client.Config.Name] = client
		m.mutex.Unlock()

		if client.Connected {
			successfulConnections++
			fmt.Printf("✓ Connected to cluster: %s\n", client.Config.Name)
		} else {
			connectionErrors = append(connectionErrors,
				fmt.Sprintf("Failed to connect to %s: %v", client.Config.Name, client.Error))
			fmt.Printf("✗ Failed to connect to cluster: %s (%v)\n", client.Config.Name, client.Error)
		}
	}

	// We require at least one successful connection
	if successfulConnections == 0 {
		return fmt.Errorf("failed to connect to any clusters:\n%s",
			strings.Join(connectionErrors, "\n"))
	}

	if len(connectionErrors) > 0 {
		fmt.Printf("\nWarning: Some clusters are unavailable:\n%s\n\n",
			strings.Join(connectionErrors, "\n"))
	}

	return nil
}

// connectToCluster establishes a connection to a single cluster
// This handles the complex process of loading kubeconfig and creating a client
func (m *Manager) connectToCluster(clusterConfig config.ClusterConfig) *ClusterClient {
	client := &ClusterClient{
		Config:    clusterConfig,
		Connected: false,
	}

	// Step 1: Determine which kubeconfig file to use
	kubeconfigPath := clusterConfig.KubeConfig
	if kubeconfigPath == "" {
		// Default to standard kubeconfig location
		homeDir, err := os.UserHomeDir()
		if err != nil {
			client.Error = fmt.Errorf("cannot determine home directory: %w", err)
			return client
		}
		kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
	}

	// Handle tilde expansion for paths like "~/.kube/config"
	if strings.HasPrefix(kubeconfigPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			client.Error = fmt.Errorf("cannot expand tilde in path: %w", err)
			return client
		}
		kubeconfigPath = filepath.Join(homeDir, kubeconfigPath[2:])
	}

	// Step 2: Load the kubeconfig file and create REST config
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: clusterConfig.Context},
	).ClientConfig()

	if err != nil {
		client.Error = fmt.Errorf("failed to load kubeconfig: %w", err)
		return client
	}

	// Step 3: Set timeouts for better reliability
	timeout := time.Duration(m.config.Timeout) * time.Second
	restConfig.Timeout = timeout

	// Step 4: Create the Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		client.Error = fmt.Errorf("failed to create Kubernetes client: %w", err)
		return client
	}

	// Step 5: Test the connection by trying to get cluster version
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		client.Error = fmt.Errorf("failed to connect to cluster: %w", err)
		return client
	}

	// Success! Store the working client
	client.RestConfig = restConfig
	client.Clientset = clientset
	client.Connected = true

	return client
}

// GetClient returns a client for the specified cluster
// This is like looking up a phone number and getting the active line
func (m *Manager) GetClient(clusterName string) (*ClusterClient, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	client, exists := m.clients[clusterName]
	if !exists {
		return nil, fmt.Errorf("cluster '%s' not found in configuration", clusterName)
	}

	if !client.Connected {
		return nil, fmt.Errorf("cluster '%s' is not connected: %v", clusterName, client.Error)
	}

	return client, nil
}

// GetDefaultClient returns the client for the default cluster
func (m *Manager) GetDefaultClient() (*ClusterClient, error) {
	for _, clusterConfig := range m.config.Clusters {
		if clusterConfig.IsDefault {
			return m.GetClient(clusterConfig.Name)
		}
	}

	// If no default is set, return the first available cluster
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, client := range m.clients {
		if client.Connected {
			return client, nil
		}
	}

	return nil, fmt.Errorf("no connected clusters available")
}

// ListClusters returns information about all configured clusters
func (m *Manager) ListClusters() []ClusterStatus {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var clusters []ClusterStatus
	for _, client := range m.clients {
		status := ClusterStatus{
			Name:        client.Config.Name,
			Environment: client.Config.Environment,
			Region:      client.Config.Region,
			Connected:   client.Connected,
			IsDefault:   client.Config.IsDefault,
		}

		if client.Error != nil {
			status.Error = client.Error.Error()
		}

		clusters = append(clusters, status)
	}

	return clusters
}

// ClusterStatus represents the status of a cluster connection
type ClusterStatus struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
	Region      string `json:"region"`
	Connected   bool   `json:"connected"`
	IsDefault   bool   `json:"isDefault"`
	Error       string `json:"error,omitempty"`
}

// TestConnections verifies all cluster connections are still healthy
// This is like checking if all your phone lines are still working
func (m *Manager) TestConnections() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var errors []string
	for name, client := range m.clients {
		if !client.Connected {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := client.Clientset.Discovery().ServerVersion()
		cancel()

		if err != nil {
			errors = append(errors, fmt.Sprintf("Cluster %s: %v", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("connection test failed:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}
