package cluster

import (
	"github.com/celikgo/autoz-control-tower/internal/config"
	"testing"
)

func TestNewManager(t *testing.T) {
	// Test with empty config (should fail)
	emptyConfig := &config.MultiClusterConfig{
		Clusters: []config.ClusterConfig{},
	}

	_, err := NewManager(emptyConfig)
	if err == nil {
		t.Error("Expected error with empty config, got nil")
	}
}

func TestClusterStatus(t *testing.T) {
	// Test cluster status functionality
	status := ClusterStatus{
		Name:        "test-cluster",
		Environment: "test",
		Connected:   true,
		IsDefault:   true,
	}

	if status.Name != "test-cluster" {
		t.Errorf("Expected name 'test-cluster', got '%s'", status.Name)
	}

	if !status.Connected {
		t.Error("Expected cluster to be connected")
	}
}
