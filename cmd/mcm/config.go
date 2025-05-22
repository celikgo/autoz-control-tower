package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// newConfigCmd creates the config command for managing tool configuration
// This is like providing a "settings app" for our multi-cluster manager
// Configuration management is crucial because it determines which clusters
// the tool can connect to and how it behaves
func newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage multi-cluster configuration",
		Long: `The config command helps you set up and manage the configuration file
that defines which Kubernetes clusters your multi-cluster manager can access.

Configuration is the foundation of multi-cluster operations. Without proper
configuration, the tool cannot connect to your clusters or perform any operations.
The config file defines:

- Which clusters you have access to
- How to authenticate with each cluster (kubeconfig files and contexts)
- Organizational information (environment, region) for each cluster
- Default settings like namespace and timeout values

This command provides several utilities to make configuration management easier:

1. Initialize new configuration files with examples
2. Validate existing configurations to catch problems early
3. Show current configuration in different formats
4. Help troubleshoot connectivity issues

The configuration follows a simple YAML format that's easy to edit by hand
but also provides validation to catch common mistakes before they cause
connection failures.

Examples:
  mcm config init                    # Create a sample configuration file
  mcm config show                    # Display current configuration
  mcm config validate                # Check configuration for errors
  mcm config path                    # Show where config file is located`,
	}

	// Add subcommands for different configuration operations
	configCmd.AddCommand(newConfigInitCmd())
	configCmd.AddCommand(newConfigShowCmd())
	configCmd.AddCommand(newConfigValidateCmd())
	configCmd.AddCommand(newConfigPathCmd())

	return configCmd
}

// newConfigInitCmd creates the 'config init' subcommand
// This is like a "setup wizard" that helps users create their first configuration file
func newConfigInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new configuration file",
		Long: `Create a new configuration file with sample cluster definitions.
This command helps you get started by creating a template configuration file
that you can customize for your specific clusters.

The generated file includes:
- Sample cluster definitions for common scenarios (dev, staging, prod)
- Explanatory comments for each configuration option
- Best practices for organizing multi-cluster configurations
- Security reminders about protecting kubeconfig files

After running this command, you'll need to:
1. Edit the generated file to match your actual clusters
2. Update the kubectl context names to match your kubeconfig
3. Set the correct paths to your kubeconfig files
4. Test the configuration with 'mcm clusters list'

The file will be created in the standard configuration location, following
XDG Base Directory Specification on Linux and appropriate conventions on
other operating systems.`,

		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine where to create the configuration file
			configPath, err := getConfigInitPath()
			if err != nil {
				return fmt.Errorf("failed to determine config path: %w", err)
			}

			// Check if config file already exists to avoid overwriting
			if _, err := os.Stat(configPath); err == nil {
				overwrite, _ := cmd.Flags().GetBool("force")
				if !overwrite {
					return fmt.Errorf("configuration file already exists at %s\nUse --force to overwrite", configPath)
				}
				fmt.Printf("⚠️  Overwriting existing configuration file at %s\n", configPath)
			}

			// Create the directory if it doesn't exist
			configDir := filepath.Dir(configPath)
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("failed to create config directory %s: %w", configDir, err)
			}

			// Read the sample configuration template
			// In a real implementation, you might embed this in the binary
			// or generate it dynamically based on detected kubeconfig contexts
			sampleConfig := generateSampleConfig()

			// Write the configuration file
			if err := os.WriteFile(configPath, []byte(sampleConfig), 0644); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			fmt.Printf("✅ Configuration file created at: %s\n\n", configPath)
			fmt.Println("Next steps:")
			fmt.Println("1. Edit the configuration file to match your clusters")
			fmt.Println("2. Update kubectl context names and kubeconfig paths")
			fmt.Println("3. Test the configuration: mcm config validate")
			fmt.Println("4. List your clusters: mcm clusters list")
			fmt.Println("\nFor help editing the configuration, see the comments in the generated file.")

			return nil
		},
	}

	cmd.Flags().Bool("force", false, "overwrite existing configuration file")
	return cmd
}

// newConfigShowCmd creates the 'config show' subcommand
// This displays the current configuration in a readable format
func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display current configuration",
		Long: `Show the current multi-cluster configuration in a readable format.
This command is useful for:
- Verifying which clusters are configured
- Checking configuration syntax before making changes  
- Documenting your current setup
- Troubleshooting configuration issues

The output shows all configured clusters with their connection details,
environment classifications, and other metadata. Sensitive information
like authentication tokens are not displayed for security reasons.`,

		RunE: func(cmd *cobra.Command, args []string) error {
			if appConfig == nil {
				return fmt.Errorf("no configuration loaded - run 'mcm config init' to create one")
			}

			fmt.Printf("Multi-Cluster Manager Configuration\n")
			fmt.Printf("===================================\n\n")

			fmt.Printf("Default Namespace: %s\n", appConfig.DefaultNamespace)
			fmt.Printf("Connection Timeout: %d seconds\n", appConfig.Timeout)
			fmt.Printf("Total Clusters: %d\n\n", len(appConfig.Clusters))

			if len(appConfig.Clusters) == 0 {
				fmt.Println("No clusters configured.")
				return nil
			}

			fmt.Println("Configured Clusters:")
			fmt.Println("-------------------")

			for i, cluster := range appConfig.Clusters {
				fmt.Printf("%d. %s\n", i+1, cluster.Name)
				fmt.Printf("   Context: %s\n", cluster.Context)
				fmt.Printf("   Environment: %s\n", getValueOrDefault(cluster.Environment, "not specified"))
				fmt.Printf("   Region: %s\n", getValueOrDefault(cluster.Region, "not specified"))
				fmt.Printf("   Kubeconfig: %s\n", getValueOrDefault(cluster.KubeConfig, "default (~/.kube/config)"))

				if cluster.IsDefault {
					fmt.Printf("   Default: ⭐ Yes\n")
				}

				fmt.Println()
			}

			return nil
		},
	}
}

// newConfigValidateCmd creates the 'config validate' subcommand
// This checks the configuration for common problems and connectivity issues
func newConfigValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration and test cluster connectivity",
		Long: `Validate the multi-cluster configuration file and test connectivity to all clusters.
This command performs comprehensive validation including:

1. Configuration file syntax and structure validation
2. Required field presence checking
3. Duplicate cluster name detection
4. Kubeconfig file existence verification
5. Kubectl context validation
6. Actual cluster connectivity testing

This is particularly useful when:
- Setting up the tool for the first time
- Adding new clusters to your configuration
- Troubleshooting connectivity issues
- Verifying configuration after changes

The validation process will report specific errors and suggestions for fixing
any problems it discovers. This helps ensure your configuration will work
reliably for actual operations.`,

		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Validating multi-cluster configuration...")
			fmt.Println()

			// The configuration validation happens during app initialization
			// If we got this far, basic validation already passed
			if appConfig == nil {
				return fmt.Errorf("no configuration could be loaded")
			}

			fmt.Printf("✅ Configuration file syntax is valid\n")
			fmt.Printf("✅ Found %d cluster(s) defined\n", len(appConfig.Clusters))

			// Test cluster connectivity (this was done during initialization)
			if clusterManager == nil {
				return fmt.Errorf("cluster manager not initialized")
			}

			fmt.Println("\nTesting cluster connectivity...")

			clusterStatuses := clusterManager.ListClusters()
			connectedCount := 0

			for _, status := range clusterStatuses {
				if status.Connected {
					connectedCount++
					fmt.Printf("✅ %s: Connected successfully\n", status.Name)
				} else {
					fmt.Printf("❌ %s: Connection failed - %s\n", status.Name, status.Error)
				}
			}

			fmt.Println()

			if connectedCount == len(clusterStatuses) {
				fmt.Printf("🎉 All %d clusters are connected and ready!\n", connectedCount)
				fmt.Println("\nYour configuration is working perfectly. You can now use:")
				fmt.Println("- mcm clusters list       # View cluster status")
				fmt.Println("- mcm deployments list    # View deployments across clusters")
				fmt.Println("- mcm pods list           # View pods across clusters")
			} else {
				fmt.Printf("⚠️  %d/%d clusters connected successfully\n", connectedCount, len(clusterStatuses))
				fmt.Println("\nTroubleshooting tips for failed connections:")
				fmt.Println("- Verify kubectl context names: kubectl config get-contexts")
				fmt.Println("- Check kubeconfig file paths and permissions")
				fmt.Println("- Test individual cluster access: kubectl --context=CONTEXT_NAME get nodes")
				fmt.Println("- Ensure cluster credentials haven't expired")
			}

			return nil
		},
	}
}

// newConfigPathCmd creates the 'config path' subcommand
// This shows where the configuration file is located
func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show configuration file location",
		Long: `Display the path to the configuration file being used.
This is helpful for:
- Finding the configuration file to edit it
- Verifying which config file is being loaded
- Troubleshooting configuration issues
- Scripting and automation`,

		RunE: func(cmd *cobra.Command, args []string) error {
			// Try to determine the actual config path being used
			configPath := findConfigPath()

			if configPath == "" {
				fmt.Println("No configuration file found.")
				fmt.Println("\nThe tool looks for configuration in these locations (in order):")
				fmt.Println("1. ./mcm-config.yaml (current directory)")
				if homeDir, err := os.UserHomeDir(); err == nil {
					fmt.Printf("2. %s/.mcm/config.yaml (user home directory)\n", homeDir)

					if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
						fmt.Printf("3. %s/mcm/config.yaml (XDG config directory)\n", xdgConfig)
					} else {
						fmt.Printf("3. %s/.config/mcm/config.yaml (XDG config directory)\n", homeDir)
					}
				}
				fmt.Println("\nRun 'mcm config init' to create a configuration file.")
				return nil
			}

			// Check if the file actually exists
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				fmt.Printf("Configuration file: %s (does not exist)\n", configPath)
				fmt.Println("Run 'mcm config init' to create it.")
			} else {
				fmt.Printf("Configuration file: %s\n", configPath)

				// Show some basic file information
				if info, err := os.Stat(configPath); err == nil {
					fmt.Printf("Last modified: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))
					fmt.Printf("File size: %d bytes\n", info.Size())
				}
			}

			return nil
		},
	}
}

// Helper functions for configuration management

// getConfigInitPath determines where to create a new configuration file
func getConfigInitPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Use XDG Base Directory Specification
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = filepath.Join(homeDir, ".config")
	}

	return filepath.Join(configDir, "mcm", "config.yaml"), nil
}

// findConfigPath attempts to locate the current configuration file
func findConfigPath() string {
	// Check current directory
	if _, err := os.Stat("./mcm-config.yaml"); err == nil {
		return "./mcm-config.yaml"
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Check ~/.mcm/config.yaml
	path := filepath.Join(homeDir, ".mcm", "config.yaml")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// Check XDG config directory
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = filepath.Join(homeDir, ".config")
	}

	path = filepath.Join(configDir, "mcm", "config.yaml")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	return ""
}

// generateSampleConfig creates a sample configuration file content
func generateSampleConfig() string {
	return `# Multi-Cluster Manager Configuration
# This file defines all the Kubernetes clusters you want to manage

# Global settings
defaultNamespace: "default"
timeout: 30

# Your clusters - customize these for your environment
clusters:
  # Development cluster - usually for testing new features
  - name: "dev-cluster"
    context: "dev-context"              # kubectl context name
    kubeconfig: "~/.kube/config"        # path to kubeconfig file
    environment: "development"
    region: "us-west-2"
    default: true                       # this will be the default cluster

  # Staging cluster - final testing before production
  - name: "staging-cluster"
    context: "staging-context"
    kubeconfig: "~/.kube/config"
    environment: "staging"
    region: "us-east-1"

  # Production clusters - your live applications
  - name: "prod-us-east"
    context: "production-us-east"
    kubeconfig: "~/.kube/prod-config"   # separate kubeconfig for production
    environment: "production"
    region: "us-east-1"

  - name: "prod-eu-west"
    context: "production-eu-west"
    kubeconfig: "~/.kube/prod-config"
    environment: "production"
    region: "eu-west-1"

# Setup Instructions:
# 1. Replace the context names with your actual kubectl contexts
#    Find your contexts with: kubectl config get-contexts
# 2. Update kubeconfig paths to point to your actual files
# 3. Adjust cluster names, environments, and regions to match your setup
# 4. Test the configuration with: mcm config validate
# 5. List your clusters with: mcm clusters list

# Security Note:
# Keep your kubeconfig files secure and never commit them to version control
`
}

// getValueOrDefault returns the value if not empty, otherwise returns the default
func getValueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
