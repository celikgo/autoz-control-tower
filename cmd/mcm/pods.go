package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"

	"github.com/celikgo/autoz-control-tower/internal/workload"
)

// newPodsCmd creates the pods command with its subcommands
// Think of this as your "detailed infrastructure inspector" - while deployments show you
// the high-level application status, pods show you exactly what's running where
func newPodsCmd() *cobra.Command {
	podsCmd := &cobra.Command{
		Use:   "pods",
		Short: "Manage and view pods across clusters",
		Long: `The pods command provides detailed visibility into the actual running instances
of your applications across multiple Kubernetes clusters. While deployments tell you about
the desired state, pods show you the current reality of what's actually running.

This level of detail is essential for:
- Troubleshooting: "Why is my application slow in the EU region?"
- Capacity planning: "How are pods distributed across nodes?"
- Incident response: "Which specific pod instances are having problems?"
- Resource utilization: "Are pods restarting frequently in any cluster?"

Pod information includes:
- Exact pod names and their current status (Running, Pending, Failed, etc.)
- Container readiness (how many containers are ready vs total)
- Restart counts (indicating stability problems)
- Node placement (useful for understanding resource distribution)
- Age information (helpful for tracking deployment propagation)

The multi-cluster view is particularly powerful during deployments - you can watch
as new pods start up across all your regions and verify that the rollout is proceeding
as expected everywhere.

Examples:
  mcm pods list                                    # All pods, all clusters
  mcm pods list --clusters=prod-us                # Only specific cluster
  mcm pods list --namespace=default               # Only default namespace
  mcm pods list --selector="app=nginx"            # Filter by label selector
  mcm pods list --output=json | jq '.pods[] | select(.status=="Failed")'  # Find failed pods`,
	}

	podsCmd.AddCommand(newPodsListCmd())
	return podsCmd
}

// newPodsListCmd creates the 'pods list' subcommand
func newPodsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pods across multiple clusters",
		Long: `Display detailed pod information from all configured clusters or a subset.
This command queries multiple Kubernetes clusters in parallel to give you a comprehensive
view of all running pod instances.

Understanding pod status is crucial for operations:
- Running: Pod is executing normally (this is what you want to see)
- Pending: Pod is waiting to be scheduled (might indicate resource constraints)
- Failed: Pod has terminated with an error (requires investigation)
- Succeeded: Pod completed successfully (normal for job workloads)
- Unknown: Pod status cannot be determined (often a node communication issue)

The Ready column shows container readiness in "ready/total" format:
- "2/2" means both containers in the pod are ready
- "1/2" means only one of two containers is ready (potential problem)
- "0/1" means the single container is not yet ready

Restart counts indicate stability:
- 0 restarts: Pod has been stable since creation
- Low restart count (1-3): Possibly normal application restarts
- High restart count (10+): Indicates a problem that needs investigation

This information helps answer critical operational questions like "Are there any
unhealthy pods in production?" or "Did the deployment succeed in all regions?"`,

		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse command flags to determine query parameters
			clusters := parseClusterList(cmd.Flag("clusters").Value.String())
			namespace := cmd.Flag("namespace").Value.String()
			labelSelector := cmd.Flag("selector").Value.String()
			outputFormat := viper.GetString("output")

			// Query all clusters for pod information in parallel
			pods, err := workloadManager.ListPods(clusters, namespace, labelSelector)
			if err != nil {
				return fmt.Errorf("failed to list pods: %w", err)
			}

			// Sort pods for consistent, scannable output
			// Primary sort: cluster name (group by infrastructure)
			// Secondary sort: namespace (group by application boundary)
			// Tertiary sort: pod name (alphabetical within namespace)
			sort.Slice(pods, func(i, j int) bool {
				if pods[i].ClusterName != pods[j].ClusterName {
					return pods[i].ClusterName < pods[j].ClusterName
				}
				if pods[i].Namespace != pods[j].Namespace {
					return pods[i].Namespace < pods[j].Namespace
				}
				return pods[i].Name < pods[j].Name
			})

			// Output in requested format
			switch outputFormat {
			case "json":
				return outputPodsJSON(pods)
			case "yaml":
				return outputPodsYAML(pods)
			default:
				return outputPodsTable(pods)
			}
		},
	}

	// Add flags for filtering and targeting specific pods
	cmd.Flags().String("clusters", "", "comma-separated list of cluster names")
	cmd.Flags().StringP("namespace", "n", "", "namespace to list pods from")
	cmd.Flags().StringP("selector", "l", "", "label selector to filter pods (e.g., 'app=nginx,tier=frontend')")

	return cmd
}

// outputPodsTable displays pod information in a readable table format
// This is optimized for quick visual scanning to spot problems
func outputPodsTable(pods []workload.PodInfo) error {
	if len(pods) == 0 {
		fmt.Println("No pods found in the specified clusters and namespaces.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	// Headers that provide the most critical pod information at a glance
	fmt.Fprintln(w, "CLUSTER\tNAMESPACE\tNAME\tREADY\tSTATUS\tRESTARTS\tAGE\tNODE")
	fmt.Fprintln(w, "-------\t---------\t----\t-----\t------\t--------\t---\t----")

	for _, pod := range pods {
		// Handle error cases where we couldn't retrieve pod information
		if strings.Contains(pod.Status, "Failed to") || strings.Contains(pod.Name, "error") {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				pod.ClusterName,
				"-",
				"ERROR",
				"-",
				"❌ "+pod.Status,
				"-",
				"-",
				"-",
			)
			continue
		}

		// Add visual indicators for pod status to make problems immediately visible
		var statusIcon string
		switch pod.Status {
		case "Running":
			statusIcon = "✅ " + pod.Status
		case "Pending":
			statusIcon = "⏳ " + pod.Status
		case "Failed":
			statusIcon = "❌ " + pod.Status
		case "Succeeded":
			statusIcon = "✅ " + pod.Status
		case "Unknown":
			statusIcon = "❓ " + pod.Status
		default:
			statusIcon = pod.Status
		}

		// Highlight high restart counts as they indicate instability
		restarts := fmt.Sprintf("%d", pod.Restarts)
		if pod.Restarts > 5 {
			restarts = "⚠️ " + restarts // Warning for moderate restart counts
		}
		if pod.Restarts > 20 {
			restarts = "🚨 " + restarts // Alert for high restart counts
		}

		// Truncate long pod names to keep table readable while preserving key info
		// Pod names often include deployment names and random suffixes
		podName := pod.Name
		if len(podName) > 35 {
			// Try to preserve the meaningful prefix and show it's truncated
			podName = podName[:32] + "..."
		}

		// Truncate node names since they're often very long in cloud environments
		nodeName := pod.Node
		if len(nodeName) > 20 {
			nodeName = nodeName[:17] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			pod.ClusterName,
			pod.Namespace,
			podName,
			pod.Ready,
			statusIcon,
			restarts,
			pod.Age,
			nodeName,
		)
	}

	// Provide summary statistics to give context
	runningCount := countPodsByStatus(pods, "Running")
	totalCount := len(pods)
	clusterCount := countUniquePodClusters(pods)

	fmt.Printf("\nFound %d pods (%d running) across %d clusters\n",
		totalCount, runningCount, clusterCount)

	// Highlight if there are any non-running pods as this might need attention
	if runningCount < totalCount {
		nonRunning := totalCount - runningCount
		fmt.Printf("⚠️  Note: %d pods are not in Running state - this may require investigation\n", nonRunning)
	}

	return nil
}

// outputPodsJSON formats pod information as JSON for programmatic use
func outputPodsJSON(pods []workload.PodInfo) error {
	output := struct {
		Pods     []workload.PodInfo `json:"pods"`
		Count    int                `json:"count"`
		Clusters []string           `json:"clusters"`
		Summary  PodSummary         `json:"summary"`
	}{
		Pods:     pods,
		Count:    len(pods),
		Clusters: getUniquePodClusters(pods),
		Summary:  generatePodSummary(pods),
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pods to JSON: %w", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

// outputPodsYAML formats pod information as YAML
func outputPodsYAML(pods []workload.PodInfo) error {
	output := struct {
		Pods     []workload.PodInfo `yaml:"pods"`
		Count    int                `yaml:"count"`
		Clusters []string           `yaml:"clusters"`
		Summary  PodSummary         `yaml:"summary"`
	}{
		Pods:     pods,
		Count:    len(pods),
		Clusters: getUniquePodClusters(pods),
		Summary:  generatePodSummary(pods),
	}

	yamlData, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal pods to YAML: %w", err)
	}

	fmt.Print(string(yamlData))
	return nil
}

// PodSummary provides aggregate statistics about the pod collection
// This is useful for understanding the overall health of your infrastructure
type PodSummary struct {
	Running   int `json:"running" yaml:"running"`
	Pending   int `json:"pending" yaml:"pending"`
	Failed    int `json:"failed" yaml:"failed"`
	Succeeded int `json:"succeeded" yaml:"succeeded"`
	Unknown   int `json:"unknown" yaml:"unknown"`
	Other     int `json:"other" yaml:"other"`
}

// generatePodSummary calculates summary statistics from the pod list
func generatePodSummary(pods []workload.PodInfo) PodSummary {
	summary := PodSummary{}

	for _, pod := range pods {
		switch pod.Status {
		case "Running":
			summary.Running++
		case "Pending":
			summary.Pending++
		case "Failed":
			summary.Failed++
		case "Succeeded":
			summary.Succeeded++
		case "Unknown":
			summary.Unknown++
		default:
			summary.Other++
		}
	}

	return summary
}

// countPodsByStatus counts pods in a specific status
func countPodsByStatus(pods []workload.PodInfo, status string) int {
	count := 0
	for _, pod := range pods {
		if pod.Status == status {
			count++
		}
	}
	return count
}

// countUniquePodClusters counts unique clusters in the pod list
func countUniquePodClusters(pods []workload.PodInfo) int {
	clusters := make(map[string]bool)
	for _, pod := range pods {
		clusters[pod.ClusterName] = true
	}
	return len(clusters)
}

// getUniquePodClusters returns sorted list of unique cluster names
func getUniquePodClusters(pods []workload.PodInfo) []string {
	clusterSet := make(map[string]bool)
	for _, pod := range pods {
		clusterSet[pod.ClusterName] = true
	}

	var clusters []string
	for cluster := range clusterSet {
		clusters = append(clusters, cluster)
	}

	sort.Strings(clusters)
	return clusters
}
