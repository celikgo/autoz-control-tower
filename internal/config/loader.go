package config

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

// LoadConfig reads the multi-cluster configuration from a YAML file
// This function is like opening your address book and reading all the contacts
func LoadConfig(configPath string) (*MultiClusterConfig, error) {
	// If no config path provided, try to find it in common locations
	if configPath == "" {
		configPath = findDefaultConfigPath()
	}

	// Read the YAML file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Parse YAML into our config structure
	var config MultiClusterConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Validate the configuration before returning it
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Set default values for any missing fields
	setDefaults(&config)

	return &config, nil
}

// findDefaultConfigPath looks for config file in standard locations
// This follows the XDG specification and common practices
func findDefaultConfigPath() string {
	// Check for config in current directory first
	if _, err := os.Stat("./mcm-config.yaml"); err == nil {
		return "./mcm-config.yaml"
	}

	// Check user's home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(homeDir, ".mcm", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// Check XDG config directory
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" && homeDir != "" {
		configDir = filepath.Join(homeDir, ".config")
	}

	if configDir != "" {
		configPath := filepath.Join(configDir, "mcm", "config.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// Return default path if nothing found
	if homeDir != "" {
		return filepath.Join(homeDir, ".mcm", "config.yaml")
	}

	return "./mcm-config.yaml"
}

// validateConfig ensures the configuration makes sense
// This is like double-checking that all your addresses have valid zip codes
func validateConfig(config *MultiClusterConfig) error {
	if len(config.Clusters) == 0 {
		return fmt.Errorf("no clusters defined in configuration")
	}

	clusterNames := make(map[string]bool)
	defaultCount := 0

	for i, cluster := range config.Clusters {
		// Check for required fields
		if cluster.Name == "" {
			return fmt.Errorf("cluster at index %d has no name", i)
		}

		if cluster.Context == "" {
			return fmt.Errorf("cluster '%s' has no context specified", cluster.Name)
		}

		// Check for duplicate names
		if clusterNames[cluster.Name] {
			return fmt.Errorf("duplicate cluster name: %s", cluster.Name)
		}
		clusterNames[cluster.Name] = true

		// Count default clusters
		if cluster.IsDefault {
			defaultCount++
		}

		// Validate kubeconfig path exists if specified
		if cluster.KubeConfig != "" {
			if _, err := os.Stat(cluster.KubeConfig); err != nil {
				return fmt.Errorf("kubeconfig file not found for cluster '%s': %s", cluster.Name, cluster.KubeConfig)
			}
		}
	}

	// Warn if more than one default cluster (we'll use the first one)
	if defaultCount > 1 {
		fmt.Fprintf(os.Stderr, "Warning: Multiple clusters marked as default. Using the first one.\n")
	}

	return nil
}

// setDefaults fills in reasonable default values for missing configuration
func setDefaults(config *MultiClusterConfig) {
	// Set default namespace if not specified
	if config.DefaultNamespace == "" {
		config.DefaultNamespace = "default"
	}

	// Set default timeout if not specified (30 seconds)
	if config.Timeout == 0 {
		config.Timeout = 30
	}

	// If no cluster is marked as default, mark the first one
	hasDefault := false
	for _, cluster := range config.Clusters {
		if cluster.IsDefault {
			hasDefault = true
			break
		}
	}

	if !hasDefault && len(config.Clusters) > 0 {
		config.Clusters[0].IsDefault = true
	}
}
