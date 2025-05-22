package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/celikgo/autoz-control-tower/internal/cluster"
	"github.com/celikgo/autoz-control-tower/internal/config"
	"github.com/celikgo/autoz-control-tower/internal/workload"
)

// Global variables to hold our core components
// These will be initialized once and reused across all commands
var (
	clusterManager  *cluster.Manager
	workloadManager *workload.Manager
	appConfig       *config.MultiClusterConfig
)

// rootCmd represents the base command when called without any subcommands
// This is the foundation of our CLI - everything branches from here
var rootCmd = &cobra.Command{
	Use:   "mcm",
	Short: "Multi-Cluster Manager for Kubernetes",
	Long: `MCM (Multi-Cluster Manager) is a powerful CLI tool for managing Kubernetes workloads
across multiple clusters simultaneously.

With MCM, you can:
- Connect to multiple Kubernetes clusters at once
- List and manage deployments across all your environments
- Deploy applications to multiple clusters in parallel
- Monitor the health of your entire Kubernetes infrastructure

Examples:
  mcm clusters list                          # Show all configured clusters
  mcm deployments list                       # List deployments across all clusters  
  mcm deployments list --clusters=prod-us   # List deployments in specific cluster
  mcm pods list --namespace=default         # List pods across all clusters
  mcm deploy app.yaml --clusters=prod-us,prod-eu  # Deploy to multiple clusters

Configuration:
  MCM looks for configuration in these locations (in order):
  1. ./mcm-config.yaml (current directory)
  2. ~/.mcm/config.yaml (user home directory)
  3. $XDG_CONFIG_HOME/mcm/config.yaml (XDG config directory)

  Use 'mcm config init' to create a sample configuration file.`,

	// PersistentPreRun initializes our core components before any command runs
	// This is like "starting the engine" before driving - we establish all cluster
	// connections upfront so individual commands execute quickly
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize configuration
		configPath := viper.GetString("config")
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		appConfig = cfg

		// Initialize cluster manager (this establishes all cluster connections)
		fmt.Printf("Connecting to clusters...\n")
		mgr, err := cluster.NewManager(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize cluster manager: %w", err)
		}
		clusterManager = mgr

		// Initialize workload manager
		workloadManager = workload.NewManager(clusterManager)

		return nil
	},
}

func main() {
	// Execute the root command - this starts the entire CLI application
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Configure Cobra and Viper for command-line flag and config file handling
	cobra.OnInitialize(initConfig)

	// Global flags that apply to all commands
	rootCmd.PersistentFlags().String("config", "", "config file path (default: auto-detect)")
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().String("output", "table", "output format (table, json, yaml)")

	// Bind flags to viper for configuration management
	// We check these errors because flag binding can fail if flag names don't match
	// or if the viper configuration is in an invalid state
	if err := viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config")); err != nil {
		// This is a programming error that should be caught during development
		// We use panic here because this indicates a fundamental setup problem
		panic(fmt.Sprintf("failed to bind config flag: %v", err))
	}
	if err := viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose")); err != nil {
		panic(fmt.Sprintf("failed to bind verbose flag: %v", err))
	}
	if err := viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output")); err != nil {
		panic(fmt.Sprintf("failed to bind output flag: %v", err))
	}

	// Add all our subcommands to the root command
	// This builds the complete command tree that users will interact with
	rootCmd.AddCommand(newClustersCmd())
	rootCmd.AddCommand(newDeploymentsCmd())
	rootCmd.AddCommand(newPodsCmd())
	rootCmd.AddCommand(newDeployCmd())
	rootCmd.AddCommand(newConfigCmd())
}

// initConfig reads in config file and ENV variables if set
func initConfig() {
	// Set up viper to automatically read environment variables
	// This allows users to override config with environment variables like MCM_CONFIG
	viper.SetEnvPrefix("MCM")
	viper.AutomaticEnv()

	// If a config file is found, read it in
	configFile := viper.GetString("config")
	if configFile != "" {
		viper.SetConfigFile(configFile)
		if err := viper.ReadInConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not read config file: %v\n", err)
		}
	}
}
