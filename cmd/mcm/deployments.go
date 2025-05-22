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

// newDeploymentsCmd creates the deployments command with all its subcommands
// This is like building a "deployment control panel" that works across all your clusters
func newDeploymentsCmd() *cobra.Command {
	deploymentsCmd := &cobra.Command{
		Use:   "deployments",
		Short: "Manage and view deployments across clusters",
		Long: `The deployments command provides comprehensive management of Kubernetes deployments
across multiple clusters. This is essential for understanding the state of your applications
when they're distributed across different environments, regions, or availability zones.

Key capabilities:
- View all deployments across multiple clusters simultaneously
- Filter by specific clusters, namespaces, or deployment names  
- See deployment health, replica counts, and image versions at a glance
- Compare deployment states across environments (dev vs staging vs prod)
- Export deployment information for reporting or automation

This command is particularly powerful for:
- Daily operations: "Are all my applications healthy across all regions?"
- Deployments: "Did my new version deploy successfully to all production clusters?"
- Troubleshooting: "Which clusters have the old version of my application?"
- Compliance: "Are all environments running the approved image versions?"

Examples:
  mcm deployments list                              # All deployments, all clusters
  mcm deployments list --clusters=prod-us,prod-eu  # Only production clusters
  mcm deployments list --namespace=kube-system     # System deployments only
  mcm deployments list --output=json               # Machine-readable output`,
	}

	// Add the list subcommand - this is the primary operation most users will use
	deploymentsCmd.AddCommand(newDeploymentsListCmd())

	return deploymentsCmd
}

// newDeploymentsListCmd creates the 'deployments list' subcommand
// This is where the real magic happens - showing deployment status across multiple clusters
func newDeploymentsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List deployments across multiple clusters",
		Long: `Display deployment information from all configured clusters or a subset of clusters.
This command queries multiple Kubernetes clusters in parallel and presents a unified view
of your deployment landscape.

The output includes critical information for operations:
- Deployment name and namespace for identification
- Current replica count vs desired replica count (health indicator)
- Container image version (crucial for version tracking)
- Overall status (Ready, Partial, NotReady)
- Age of the deployment (useful for change tracking)
- Which cluster the deployment is running in

Understanding the status indicators:
- Ready: All replicas are running and healthy
- Partial: Some replicas are running, but not all desired replicas are ready
- NotReady: No replicas are currently ready (likely a problem)

This unified view is incredibly valuable because it answers questions like:
"Are all my production applications healthy?" or "Did my deployment succeed in all regions?"
without requiring you to manually check each cluster individually.`,

		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse command-line flags to determine what to show
			clusters := parseClusterList(cmd.Flag("clusters").Value.String())
			namespace := cmd.Flag("namespace").Value.String()
			outputFormat := viper.GetString("output")

			// Query all specified clusters for deployment information
			// This happens in parallel, so even querying 10+ clusters is fast
			deployments, err := workloadManager.ListDeployments(clusters, namespace)
			if err != nil {
				return fmt.Errorf("failed to list deployments: %w", err)
			}

			// Sort deployments for consistent output
			// We sort by cluster name first, then by namespace, then by deployment name
			// This makes it easy to scan the output and find specific deployments
			sort.Slice(deployments, func(i, j int) bool {
				if deployments[i].ClusterName != deployments[j].ClusterName {
					return deployments[i].ClusterName < deployments[j].ClusterName
				}
				if deployments[i].Namespace != deployments[j].Namespace {
					return deployments[i].Namespace < deployments[j].Namespace
				}
				return deployments[i].Name < deployments[j].Name
			})

			// Output in the requested format
			switch outputFormat {
			case "json":
				return outputDeploymentsJSON(deployments)
			case "yaml":
				return outputDeploymentsYAML(deployments)
			default:
				return outputDeploymentsTable(deployments)
			}
		},
	}

	// Add flags specific to the deployments list command
	// These give users fine-grained control over what they want to see
	cmd.Flags().String("clusters", "", "comma-separated list of cluster names (default: all clusters)")
	cmd.Flags().StringP("namespace", "n", "", "namespace to list deployments from (default: all namespaces)")

	return cmd
}

// outputDeploymentsTable displays deployment information in a human-readable table
// This is the most common output format - designed for quick visual scanning
func outputDeploymentsTable(deployments []workload.DeploymentInfo) error {
	if len(deployments) == 0 {
		fmt.Println("No deployments found in the specified clusters and namespaces.")
		return nil
	}

	// Create a tab-aligned table writer for professional-looking output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	// Print table headers - these provide context for each column
	fmt.Fprintln(w, "CLUSTER\tNAMESPACE\tNAME\tREPLICAS\tSTATUS\tIMAGE\tAGE")
	fmt.Fprintln(w, "-------\t---------\t----\t--------\t------\t-----\t---")

	for _, deployment := range deployments {
		// Handle error cases gracefully - show what we can, indicate what failed
		if deployment.Error != "" {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				deployment.ClusterName,
				"-",
				"ERROR",
				"-",
				"❌ "+deployment.Error,
				"-",
				"-",
			)
			continue
		}

		// Format the replica information to show current vs desired
		// This is crucial for understanding deployment health at a glance
		replicas := fmt.Sprintf("%d/%d", deployment.ReadyReplicas, deployment.Replicas)

		// Add visual indicators for deployment status
		// These make it easy to quickly spot problems in a long list
		var statusIcon string
		switch deployment.Status {
		case "Ready":
			statusIcon = "✅ " + deployment.Status
		case "Partial":
			statusIcon = "⚠️  " + deployment.Status
		case "NotReady":
			statusIcon = "❌ " + deployment.Status
		default:
			statusIcon = "❓ " + deployment.Status
		}

		// Truncate long image names to keep the table readable
		// Full image names can be very long with registry URLs and SHA digests
		image := deployment.Image
		if len(image) > 40 {
			// Keep the image name but truncate the middle part
			// This preserves the most important parts (registry and tag)
			parts := strings.Split(image, "/")
			if len(parts) > 1 {
				image = parts[0] + "/..." + parts[len(parts)-1]
			}
			if len(image) > 40 {
				image = image[:37] + "..."
			}
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			deployment.ClusterName,
			deployment.Namespace,
			deployment.Name,
			replicas,
			statusIcon,
			image,
			deployment.Age,
		)
	}

	// Print a summary line to give context about what was shown
	fmt.Printf("\nFound %d deployments across %d clusters\n",
		len(deployments), countUniqueClusters(deployments))

	return nil
}

// outputDeploymentsJSON formats deployment information as JSON
// This is useful for automation, scripting, or integration with other tools
func outputDeploymentsJSON(deployments []workload.DeploymentInfo) error {
	// Wrap the deployments in a structure that provides metadata
	// This makes the JSON output more useful for programmatic consumption
	output := struct {
		Deployments []workload.DeploymentInfo `json:"deployments"`
		Count       int                       `json:"count"`
		Clusters    []string                  `json:"clusters"`
	}{
		Deployments: deployments,
		Count:       len(deployments),
		Clusters:    getUniqueClusters(deployments),
	}

	// Use indented JSON for readability when humans are viewing it
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal deployments to JSON: %w", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

// outputDeploymentsYAML formats deployment information as YAML
// Some users prefer YAML for its readability and comments support
func outputDeploymentsYAML(deployments []workload.DeploymentInfo) error {
	output := struct {
		Deployments []workload.DeploymentInfo `yaml:"deployments"`
		Count       int                       `yaml:"count"`
		Clusters    []string                  `yaml:"clusters"`
	}{
		Deployments: deployments,
		Count:       len(deployments),
		Clusters:    getUniqueClusters(deployments),
	}

	yamlData, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal deployments to YAML: %w", err)
	}

	fmt.Print(string(yamlData))
	return nil
}

// parseClusterList converts a comma-separated string into a slice of cluster names
// This handles user input like "prod-us,prod-eu,staging" and cleans it up
func parseClusterList(clusterString string) []string {
	if clusterString == "" {
		return nil // Return nil to indicate "all clusters"
	}

	// Split by comma and clean up each cluster name
	clusters := strings.Split(clusterString, ",")
	var result []string
	for _, cluster := range clusters {
		cluster = strings.TrimSpace(cluster)
		if cluster != "" {
			result = append(result, cluster)
		}
	}

	return result
}

// countUniqueClusters counts how many different clusters are represented in the results
// This is useful for summary information
func countUniqueClusters(deployments []workload.DeploymentInfo) int {
	clusters := make(map[string]bool)
	for _, deployment := range deployments {
		clusters[deployment.ClusterName] = true
	}
	return len(clusters)
}

// getUniqueClusters returns a sorted list of unique cluster names from the deployments
// This is useful for metadata in JSON/YAML output
func getUniqueClusters(deployments []workload.DeploymentInfo) []string {
	clusterSet := make(map[string]bool)
	for _, deployment := range deployments {
		clusterSet[deployment.ClusterName] = true
	}

	var clusters []string
	for cluster := range clusterSet {
		clusters = append(clusters, cluster)
	}

	sort.Strings(clusters)
	return clusters
}
