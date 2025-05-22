package examples

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/celikgo/autoz-control-tower/internal/cluster"
	"github.com/celikgo/autoz-control-tower/internal/config"
	"github.com/celikgo/autoz-control-tower/internal/workload"
)

func TestFullWorkflow(t *testing.T) {
	// Skip if no kubeconfig available
	if os.Getenv("KUBECONFIG") == "" && !fileExists(os.ExpandEnv("$HOME/.kube/config")) {
		t.Skip("No kubeconfig available, skipping integration test")
	}

	// Create test configuration
	cfg := &config.MultiClusterConfig{
		DefaultNamespace: "default",
		Timeout:          30,
		Clusters: []config.ClusterConfig{
			{
				Name:      "test-cluster",
				Context:   getCurrentContext(t),
				IsDefault: true,
			},
		},
	}

	// Initialize managers
	clusterMgr, err := cluster.NewManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create cluster manager: %v", err)
	}

	workloadMgr := workload.NewManager(clusterMgr)

	// Test listing deployments
	deployments, err := workloadMgr.ListDeployments(nil, "")
	if err != nil {
		t.Fatalf("Failed to list deployments: %v", err)
	}

	t.Logf("Found %d deployments", len(deployments))

	// Test listing pods
	pods, err := workloadMgr.ListPods(nil, "", "")
	if err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}

	t.Logf("Found %d pods", len(pods))
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func getCurrentContext(t *testing.T) string {
	// Implementation to get current kubectl context
	return "docker-desktop" // Default for testing
}
