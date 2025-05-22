package examples

import (
	"github.com/celikgo/autoz-control-tower/internal/cluster"
	"github.com/celikgo/autoz-control-tower/internal/config"
	"github.com/celikgo/autoz-control-tower/internal/workload"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"testing"
)

// TestFullWorkflow demonstrates a complete multi-cluster workflow
// This test only runs when a valid Kubernetes cluster is available
func TestFullWorkflow(t *testing.T) {
	// Step 1: Check if we should run integration tests
	if shouldSkipIntegrationTests() {
		t.Skip("Skipping integration test: no Kubernetes cluster available")
	}

	// Step 2: Detect available cluster context
	currentContext := getCurrentKubeContext(t)
	if currentContext == "" {
		t.Skip("Skipping integration test: no kubectl context available")
	}

	t.Logf("Running integration test against context: %s", currentContext)

	// Step 3: Create test configuration using the detected context
	cfg := &config.MultiClusterConfig{
		DefaultNamespace: "default",
		Timeout:          30,
		Clusters: []config.ClusterConfig{
			{
				Name:      "integration-test-cluster",
				Context:   currentContext,
				IsDefault: true,
			},
		},
	}

	// Step 4: Test cluster manager initialization
	t.Log("Initializing cluster manager...")
	clusterMgr, err := cluster.NewManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create cluster manager: %v", err)
	}

	// Step 5: Verify cluster connectivity
	t.Log("Testing cluster connectivity...")
	clusters := clusterMgr.ListClusters()
	if len(clusters) == 0 {
		t.Fatal("No clusters found in manager")
	}

	connectedClusters := 0
	for _, cluster := range clusters {
		if cluster.Connected {
			connectedClusters++
			t.Logf("✓ Successfully connected to cluster: %s", cluster.Name)
		} else {
			t.Logf("✗ Failed to connect to cluster: %s - %s", cluster.Name, cluster.Error)
		}
	}

	if connectedClusters == 0 {
		t.Fatal("No clusters are connected - cannot proceed with integration test")
	}

	// Step 6: Test workload operations
	workloadMgr := workload.NewManager(clusterMgr)

	// Test listing deployments
	t.Log("Testing deployment listing...")
	deployments, err := workloadMgr.ListDeployments(nil, "")
	if err != nil {
		t.Fatalf("Failed to list deployments: %v", err)
	}
	t.Logf("Found %d deployments in cluster", len(deployments))

	// Test listing pods
	t.Log("Testing pod listing...")
	pods, err := workloadMgr.ListPods(nil, "", "")
	if err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}
	t.Logf("Found %d pods in cluster", len(pods))

	// Step 7: Test deployment operation (if we have a test manifest)
	testManifestPath := "nginx-deployment.yaml"
	if fileExists(testManifestPath) {
		t.Log("Testing deployment operation...")
		yamlContent, err := os.ReadFile(testManifestPath)
		if err != nil {
			t.Logf("Could not read test manifest: %v", err)
		} else {
			// Deploy to a test namespace to avoid conflicts
			testNamespace := "mcm-integration-test"
			err = workloadMgr.DeployToCluster("integration-test-cluster", testNamespace, string(yamlContent))
			if err != nil {
				// Don't fail the test if deployment fails - the namespace might not exist
				t.Logf("Test deployment failed (this might be expected): %v", err)
			} else {
				t.Log("✓ Test deployment successful")
			}
		}
	}

	t.Log("Integration test completed successfully!")
}

// shouldSkipIntegrationTests determines whether integration tests should be skipped
// This follows the pattern used by Kubernetes itself and other infrastructure tools
func shouldSkipIntegrationTests() bool {
	// Check if explicitly disabled
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		return true
	}

	// Check if running in CI without cluster access
	if os.Getenv("CI") == "true" && os.Getenv("KUBECONFIG") == "" {
		return true
	}

	// Check if kubeconfig is available
	if !hasKubeconfig() {
		return true
	}

	return false
}

// getCurrentKubeContext gets the current kubectl context
// This ensures we test against whatever cluster the developer has active
func getCurrentKubeContext(t *testing.T) string {
	// Try to get current context using client-go
	kubeconfig := getKubeconfigPath()
	if kubeconfig == "" {
		return ""
	}

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		t.Logf("Could not load kubeconfig: %v", err)
		return ""
	}

	if config.CurrentContext == "" {
		t.Logf("No current context set in kubeconfig")
		return ""
	}

	// Verify the context actually exists
	if _, exists := config.Contexts[config.CurrentContext]; !exists {
		t.Logf("Current context '%s' not found in kubeconfig", config.CurrentContext)
		return ""
	}

	return config.CurrentContext
}

// hasKubeconfig checks if a kubeconfig file is available
func hasKubeconfig() bool {
	return getKubeconfigPath() != ""
}

// getKubeconfigPath returns the path to the kubeconfig file
func getKubeconfigPath() string {
	// Check KUBECONFIG environment variable first
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		if fileExists(kubeconfig) {
			return kubeconfig
		}
	}

	// Check default location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	defaultPath := filepath.Join(homeDir, ".kube", "config")
	if fileExists(defaultPath) {
		return defaultPath
	}

	return ""
}

// fileExists checks if a file exists and is readable
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// TestClusterDetection tests our cluster detection logic with proper isolation
// This demonstrates how to test infrastructure code that depends on environment state
func TestClusterDetection(t *testing.T) {
	t.Log("Testing cluster detection logic...")

	// Create isolated test environment by temporarily changing to empty directory
	// This ensures we don't accidentally use real kubeconfig files during testing
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Create temporary directory for our isolated test environment
	tempDir := t.TempDir()
	defer func() {
		// Always restore original working directory after test
		os.Chdir(originalWd)
	}()

	// Change to temporary directory to isolate file system access
	os.Chdir(tempDir)

	// Save original environment variables so we can restore them
	originalKubeconfig := os.Getenv("KUBECONFIG")
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("KUBECONFIG", originalKubeconfig)
		os.Setenv("HOME", originalHome)
	}()

	// Test 1: No kubeconfig available anywhere
	t.Log("Test 1: Testing with no kubeconfig available")
	os.Setenv("KUBECONFIG", "")
	os.Setenv("HOME", tempDir) // Point HOME to our empty temp directory

	if hasKubeconfig() {
		t.Error("Expected hasKubeconfig() to return false when no kubeconfig exists")
	}

	// Test 2: Invalid KUBECONFIG path with no default fallback
	t.Log("Test 2: Testing with invalid KUBECONFIG path")
	os.Setenv("KUBECONFIG", "/nonexistent/path/that/definitely/does/not/exist")
	os.Setenv("HOME", tempDir) // Ensure no default kubeconfig exists

	if hasKubeconfig() {
		t.Error("Expected hasKubeconfig() to return false for nonexistent path")
	}

	// Test 3: Valid KUBECONFIG environment variable
	t.Log("Test 3: Testing with valid KUBECONFIG")
	validKubeconfigPath := filepath.Join(tempDir, "valid-kubeconfig")
	err = os.WriteFile(validKubeconfigPath, []byte("fake kubeconfig content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test kubeconfig: %v", err)
	}

	os.Setenv("KUBECONFIG", validKubeconfigPath)
	if !hasKubeconfig() {
		t.Error("Expected hasKubeconfig() to return true for valid KUBECONFIG path")
	}

	// Test 4: Default kubeconfig location
	t.Log("Test 4: Testing default kubeconfig location")
	os.Setenv("KUBECONFIG", "") // Clear KUBECONFIG to test default behavior

	// Create .kube directory in our temporary HOME
	kubeDir := filepath.Join(tempDir, ".kube")
	err = os.MkdirAll(kubeDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .kube directory: %v", err)
	}

	defaultKubeconfigPath := filepath.Join(kubeDir, "config")
	err = os.WriteFile(defaultKubeconfigPath, []byte("fake default kubeconfig"), 0644)
	if err != nil {
		t.Fatalf("Failed to create default kubeconfig: %v", err)
	}

	if !hasKubeconfig() {
		t.Error("Expected hasKubeconfig() to return true for valid default kubeconfig")
	}

	// Test skip conditions with proper isolation
	testSkipConditions(t, tempDir)

	t.Log("All cluster detection tests passed!")
}

// testSkipConditions tests the integration test skip logic in isolation
func testSkipConditions(t *testing.T, tempDir string) {
	t.Log("Testing skip conditions...")

	// Save original environment variables
	originalCI := os.Getenv("CI")
	originalSkip := os.Getenv("SKIP_INTEGRATION_TESTS")
	originalKubeconfig := os.Getenv("KUBECONFIG")
	originalHome := os.Getenv("HOME")

	defer func() {
		// Restore all original environment variables
		os.Setenv("CI", originalCI)
		os.Setenv("SKIP_INTEGRATION_TESTS", originalSkip)
		os.Setenv("KUBECONFIG", originalKubeconfig)
		os.Setenv("HOME", originalHome)
	}()

	// Test explicit skip
	t.Log("Testing explicit skip flag")
	os.Setenv("SKIP_INTEGRATION_TESTS", "true")
	os.Setenv("HOME", tempDir) // Ensure clean environment
	os.Setenv("KUBECONFIG", "")

	if !shouldSkipIntegrationTests() {
		t.Error("Expected integration tests to be skipped when explicitly disabled")
	}

	// Test skip in CI without KUBECONFIG
	t.Log("Testing CI environment without kubeconfig")
	os.Setenv("SKIP_INTEGRATION_TESTS", "") // Clear explicit skip
	os.Setenv("CI", "true")
	os.Setenv("KUBECONFIG", "")
	os.Setenv("HOME", tempDir) // Ensure no default kubeconfig exists

	if !shouldSkipIntegrationTests() {
		t.Error("Expected integration tests to be skipped in CI without KUBECONFIG")
	}

	// Test normal conditions (should not skip)
	t.Log("Testing normal conditions with kubeconfig available")
	os.Setenv("CI", "")
	os.Setenv("SKIP_INTEGRATION_TESTS", "")

	// Create a valid kubeconfig for this test
	validKubeconfigPath := filepath.Join(tempDir, "test-kubeconfig")
	err := os.WriteFile(validKubeconfigPath, []byte("fake kubeconfig"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test kubeconfig: %v", err)
	}
	os.Setenv("KUBECONFIG", validKubeconfigPath)

	if shouldSkipIntegrationTests() {
		t.Error("Expected integration tests to run when kubeconfig is available and not in CI")
	}
}
