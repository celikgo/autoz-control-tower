package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
defaultNamespace: "test"
timeout: 60
clusters:
  - name: "test-cluster"
    context: "test-context"
    environment: "test"
    default: true
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Test config loading
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Validate loaded config
	if config.DefaultNamespace != "test" {
		t.Errorf("Expected namespace 'test', got '%s'", config.DefaultNamespace)
	}

	if len(config.Clusters) != 1 {
		t.Errorf("Expected 1 cluster, got %d", len(config.Clusters))
	}

	if config.Clusters[0].Name != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", config.Clusters[0].Name)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *MultiClusterConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &MultiClusterConfig{
				Clusters: []ClusterConfig{
					{Name: "test", Context: "test-context"},
				},
			},
			wantErr: false,
		},
		{
			name: "no clusters",
			config: &MultiClusterConfig{
				Clusters: []ClusterConfig{},
			},
			wantErr: true,
		},
		{
			name: "duplicate cluster names",
			config: &MultiClusterConfig{
				Clusters: []ClusterConfig{
					{Name: "test", Context: "test-context-1"},
					{Name: "test", Context: "test-context-2"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
