package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// newDeployCmd creates the deploy command for multi-cluster deployments
// This is the "mission control" for pushing changes across your entire infrastructure
// The power here is that you can deploy to multiple clusters simultaneously,
// which is essential for maintaining consistency across regions or environments

func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [YAML_FILE]",
		Short: "Deploy YAML manifests to multiple clusters",
		Long: `Deploy Kubernetes YAML manifests to one or more clusters simultaneously.
This command is the heart of multi-cluster operations - it allows you to push
the same configuration to multiple environments, regions, or clusters in parallel.

The deploy command handles several critical scenarios that are common in production:

1. Multi-region deployments: Deploy your application to clusters in different
   geographic regions to ensure global availability and performance.

2. Environment consistency: Ensure your staging environment exactly matches
   production by deploying the same manifests to both.

3. Gradual rollouts: Deploy to a subset of clusters first, verify success,
   then deploy to remaining clusters.

4. Disaster recovery: Quickly deploy applications to backup clusters when
   your primary infrastructure is experiencing issues.

The command provides detailed feedback about each deployment, showing you
exactly which clusters succeeded and which had problems. This visibility
is crucial for understanding the state of your rollout and taking corrective
action if needed.

Safety features:
- Each cluster deployment is independent - failure in one doesn't stop others
- Detailed error reporting shows exactly what went wrong where
- Dry-run capability (planned) to preview changes before applying them
- Rollback capability (planned) to quickly revert problematic deployments

Examples:
  mcm deploy app.yaml                                    # Deploy to default cluster
  mcm deploy app.yaml --clusters=prod-us,prod-eu        # Deploy to specific clusters  
  mcm deploy app.yaml --clusters=prod-us,prod-eu --namespace=production
  mcm deploy app.yaml --all-clusters                    # Deploy to all configured clusters
  mcm deploy app.yaml --exclude=dev-cluster             # Deploy to all except specified`,

		Args: cobra.ExactArgs(1), // Require exactly one argument (the YAML file)
		RunE: func(cmd *cobra.Command, args []string) error {
			yamlFile := args[0]

			// Validate that the YAML file exists before attempting deployment
			// This prevents wasting time connecting to clusters if the file is missing
			if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
				return fmt.Errorf("YAML file not found: %s", yamlFile)
			}

			// Read the YAML file content
			yamlContent, err := os.ReadFile(yamlFile)
			if err != nil {
				return fmt.Errorf("failed to read YAML file %s: %w", yamlFile, err)
			}

			// Parse command flags to determine target clusters
			clusters, err := parseDeploymentTargets(cmd)
			if err != nil {
				return err
			}

			// Get the target namespace
			namespace := cmd.Flag("namespace").Value.String()
			if namespace == "" {
				namespace = appConfig.DefaultNamespace
			}

			fmt.Printf("Deploying %s to %d clusters...\n", yamlFile, len(clusters))
			fmt.Printf("Target clusters: %s\n", strings.Join(clusters, ", "))
			fmt.Printf("Target namespace: %s\n\n", namespace)

			// Execute the deployment across all target clusters
			// This happens in parallel, so even deploying to many clusters is fast
			results := workloadManager.DeployToMultipleClusters(clusters, namespace, string(yamlContent))

			// Analyze and report the results
			return reportDeploymentResults(results, yamlFile)
		},
	}

	// Add flags that control deployment targeting and behavior
	cmd.Flags().String("clusters", "", "comma-separated list of cluster names to deploy to")
	cmd.Flags().Bool("all-clusters", false, "deploy to all configured clusters")
	cmd.Flags().String("exclude", "", "comma-separated list of clusters to exclude (used with --all-clusters)")
	cmd.Flags().StringP("namespace", "n", "", "target namespace (default: from config)")
	// Future flags that would make this production-ready:
	// cmd.Flags().Bool("dry-run", false, "preview the deployment without applying changes")
	// cmd.Flags().Int("timeout", 300, "deployment timeout in seconds")
	// cmd.Flags().Bool("wait", false, "wait for deployment to complete before returning")

	return cmd
}

// parseDeploymentTargets determines which clusters to deploy to based on command flags
// This function handles the logic for --clusters, --all-clusters, and --exclude flags
func parseDeploymentTargets(cmd *cobra.Command) ([]string, error) {
	clustersFlag := cmd.Flag("clusters").Value.String()
	allClusters, _ := cmd.Flags().GetBool("all-clusters")
	excludeFlag := cmd.Flag("exclude").Value.String()

	// Parse the exclude list first, as it applies to multiple scenarios
	var excludeList []string
	if excludeFlag != "" {
		excludeList = parseClusterList(excludeFlag)
	}

	var targetClusters []string

	if allClusters {
		// Deploy to all configured clusters, minus any excluded ones
		allClusterStatuses := clusterManager.ListClusters()
		for _, status := range allClusterStatuses {
			if !status.Connected {
				fmt.Printf("Warning: Skipping disconnected cluster: %s\n", status.Name)
				continue
			}

			// Check if this cluster is in the exclude list
			excluded := false
			for _, excludeCluster := range excludeList {
				if status.Name == excludeCluster {
					excluded = true
					break
				}
			}

			if !excluded {
				targetClusters = append(targetClusters, status.Name)
			}
		}

		if len(excludeList) > 0 {
			fmt.Printf("Excluding clusters: %s\n", strings.Join(excludeList, ", "))
		}

	} else if clustersFlag != "" {
		// Deploy to specific clusters listed in the --clusters flag
		targetClusters = parseClusterList(clustersFlag)

		// Validate that all specified clusters are available and connected
		for _, clusterName := range targetClusters {
			client, err := clusterManager.GetClient(clusterName)
			if err != nil {
				return nil, fmt.Errorf("cluster '%s' is not available: %w", clusterName, err)
			}
			if !client.Connected {
				return nil, fmt.Errorf("cluster '%s' is not connected", clusterName)
			}
		}

	} else {
		// No specific clusters specified - use the default cluster
		defaultClient, err := clusterManager.GetDefaultClient()
		if err != nil {
			return nil, fmt.Errorf("no default cluster available and no clusters specified: %w", err)
		}
		targetClusters = []string{defaultClient.Config.Name}
	}

	if len(targetClusters) == 0 {
		return nil, fmt.Errorf("no target clusters identified for deployment")
	}

	return targetClusters, nil
}

// reportDeploymentResults analyzes deployment results and provides detailed feedback
// This function is crucial for understanding what happened during a multi-cluster deployment
func reportDeploymentResults(results map[string]error, yamlFile string) error {
	successCount := 0
	var failures []string
	var warnings []string

	fmt.Println("Deployment Results:")
	fmt.Println("==================")

	// Iterate through results and categorize outcomes
	for clusterName, err := range results {
		if err == nil {
			successCount++
			fmt.Printf("✅ %s: SUCCESS\n", clusterName)
		} else {
			// Categorize different types of errors for better user understanding
			errorMsg := err.Error()
			fmt.Printf("❌ %s: FAILED - %v\n", clusterName, err)

			// Determine if this is a warning (recoverable) or a failure (needs intervention)
			if strings.Contains(errorMsg, "already exists") || strings.Contains(errorMsg, "no changes") {
				warnings = append(warnings, fmt.Sprintf("%s: %v", clusterName, err))
			} else {
				failures = append(failures, fmt.Sprintf("%s: %v", clusterName, err))
			}
		}
	}

	fmt.Println()

	// Provide a comprehensive summary that helps users understand what to do next
	totalClusters := len(results)
	if successCount == totalClusters {
		fmt.Printf("🎉 Deployment completed successfully on all %d clusters!\n", totalClusters)
		return nil
	}

	// Report partial success scenarios
	if successCount > 0 {
		fmt.Printf("✅ Successful deployments: %d/%d clusters\n", successCount, totalClusters)
	}

	// Report warnings (things that might be okay)
	if len(warnings) > 0 {
		fmt.Printf("⚠️  Warnings (%d clusters):\n", len(warnings))
		for _, warning := range warnings {
			fmt.Printf("   %s\n", warning)
		}
		fmt.Println()
	}

	// Report failures (things that definitely need attention)
	if len(failures) > 0 {
		fmt.Printf("❌ Failures (%d clusters):\n", len(failures))
		for _, failure := range failures {
			fmt.Printf("   %s\n", failure)
		}
		fmt.Println()

		// Provide actionable guidance for common failure scenarios
		fmt.Println("Troubleshooting Tips:")
		fmt.Println("- Check cluster connectivity: mcm clusters test")
		fmt.Println("- Verify namespace exists: kubectl get namespaces")
		fmt.Println("- Check YAML syntax: kubectl apply --dry-run=client -f", yamlFile)
		fmt.Println("- Review cluster-specific differences in configuration")

		return fmt.Errorf("deployment failed on %d/%d clusters", len(failures), totalClusters)
	}

	// If we get here, we had some warnings but no hard failures
	if len(warnings) > 0 {
		fmt.Println("Deployment completed with warnings. Review the warnings above to ensure they're expected.")
	}

	return nil
}
