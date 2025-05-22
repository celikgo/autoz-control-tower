package main

import (
	"encoding/json"
	"fmt"
	"github.com/celikgo/autoz-control-tower/internal/cluster"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"
)

// newClustersCmd creates the clusters command and all its subcommands
// This is like building the "cluster management dashboard" of our tool
func newClustersCmd() *cobra.Command {
	clustersCmd := &cobra.Command{
		Use:   "clusters",
		Short: "Manage and view cluster information",
		Long: `The clusters command provides information about all configured Kubernetes clusters.
Use this to check cluster connectivity, view cluster status, and manage cluster configurations.

Examples:
  mcm clusters list                    # Show all clusters with their status
  mcm clusters test                    # Test connectivity to all clusters
  mcm clusters list --output=json     # Show cluster info in JSON format`,
	}

	// Add subcommands for different cluster operations
	clustersCmd.AddCommand(newClustersListCmd())
	clustersCmd.AddCommand(newClustersTestCmd())

	return clustersCmd
}

// newClustersListCmd creates the 'clusters list' subcommand
// This shows all configured clusters and their current connection status
func newClustersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured clusters and their status",
		Long: `Display information about all clusters defined in your configuration file.
This includes connection status, environment, region, and any connection errors.

The output shows:
- Cluster name and environment (dev, staging, prod, etc.)
- Connection status (connected/disconnected)
- Region or location information
- Whether it's marked as the default cluster
- Any error messages if connection failed`,

		RunE: func(cmd *cobra.Command, args []string) error {
			// Get cluster status information from our cluster manager
			clusters := clusterManager.ListClusters()

			// Determine output format from flags
			outputFormat := viper.GetString("output")

			switch outputFormat {
			case "json":
				return outputClustersJSON(clusters)
			case "yaml":
				return outputClustersYAML(clusters)
			default:
				return outputClustersTable(clusters)
			}
		},
	}
}

// newClustersTestCmd creates the 'clusters test' subcommand
// This actively tests connectivity to all clusters
func newClustersTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Test connectivity to all configured clusters",
		Long: `Actively test the connection to each configured cluster by making a simple API call.
This is useful for diagnosing connectivity issues or verifying that cluster credentials are working.

This command will:
- Attempt to connect to each cluster's Kubernetes API server
- Verify that authentication is working
- Report any clusters that are unreachable
- Show response times for each cluster`,

		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Testing cluster connections...")

			err := clusterManager.TestConnections()
			if err != nil {
				fmt.Printf("❌ Connection test failed:\n%v\n", err)
				return nil // Don't return error to avoid double error printing
			}

			fmt.Println("✅ All cluster connections are healthy")
			return nil
		},
	}
}

// outputClustersTable displays cluster information in a human-readable table format
// This is the default output format that most users will see
func outputClustersTable(clusters []cluster.ClusterStatus) error {
	// Create a tabwriter for nicely formatted columns
	// This is like creating a spreadsheet that auto-adjusts column widths
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	// Print table headers
	fmt.Fprintln(w, "NAME\tENVIRONMENT\tREGION\tSTATUS\tDEFAULT\tERROR")
	fmt.Fprintln(w, "----\t-----------\t------\t------\t-------\t-----")

	// Print each cluster's information
	for _, cluster := range clusters {
		// Format the status with visual indicators
		status := "❌ Disconnected"
		if cluster.Connected {
			status = "✅ Connected"
		}

		// Show if this is the default cluster
		defaultMarker := ""
		if cluster.IsDefault {
			defaultMarker = "⭐ Yes"
		}

		// Format environment and region with fallbacks
		environment := cluster.Environment
		if environment == "" {
			environment = "-"
		}

		region := cluster.Region
		if region == "" {
			region = "-"
		}

		// Truncate long error messages for table display
		errorMsg := cluster.Error
		if len(errorMsg) > 50 {
			errorMsg = errorMsg[:47] + "..."
		}
		if errorMsg == "" {
			errorMsg = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			cluster.Name,
			environment,
			region,
			status,
			defaultMarker,
			errorMsg,
		)
	}

	return nil
}

// outputClustersJSON displays cluster information in JSON format
// This is useful for programmatic consumption or integration with other tools
func outputClustersJSON(clusters []cluster.ClusterStatus) error {
	// Create a wrapper structure for better JSON organization
	output := struct {
		Clusters []cluster.ClusterStatus `json:"clusters"`
		Count    int                     `json:"count"`
	}{
		Clusters: clusters,
		Count:    len(clusters),
	}

	// Marshal to JSON with indentation for readability
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal clusters to JSON: %w", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

// outputClustersYAML displays cluster information in YAML format
// This is useful for configuration management or when YAML is preferred over JSON
func outputClustersYAML(clusters []cluster.ClusterStatus) error {
	output := struct {
		Clusters []cluster.ClusterStatus `yaml:"clusters"`
		Count    int                     `yaml:"count"`
	}{
		Clusters: clusters,
		Count:    len(clusters),
	}

	yamlData, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal clusters to YAML: %w", err)
	}

	fmt.Print(string(yamlData))
	return nil
}
