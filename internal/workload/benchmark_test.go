package workload

import (
	"testing"
	"time"
)

// BenchmarkFormatDuration tests the performance of duration formatting
// This benchmarks our utility function to ensure it remains fast even
// when called thousands of times (once per pod/deployment)
func BenchmarkFormatDuration(b *testing.B) {
	// Test with realistic duration values that we'd see in production
	testDurations := []time.Duration{
		30 * time.Second,    // Fresh pods
		5 * time.Minute,     // Recent deployments
		2 * time.Hour,       // Established workloads
		3 * 24 * time.Hour,  // Long-running services
		30 * 24 * time.Hour, // Stable infrastructure
	}

	b.ResetTimer()

	// Benchmark the actual formatting operation
	for i := 0; i < b.N; i++ {
		// Use modulo to cycle through different duration values
		// This simulates the variety of ages we'd see in real cluster data
		duration := testDurations[i%len(testDurations)]
		result := formatDuration(duration)

		// Use the result to prevent compiler optimizations from eliminating the call
		_ = result
	}
}

// BenchmarkDeploymentInfoProcessing tests processing large deployment datasets
// This simulates the performance characteristics when listing deployments
// from clusters with hundreds or thousands of deployments
func BenchmarkDeploymentInfoProcessing(b *testing.B) {
	// Create realistic deployment data that simulates what we'd get from a busy cluster
	deployments := make([]DeploymentInfo, 1000)

	for i := range deployments {
		deployments[i] = DeploymentInfo{
			ClusterName:   "benchmark-cluster",
			Namespace:     "production",
			Name:          generateDeploymentName(i),
			Replicas:      3,
			ReadyReplicas: 3,
			Image:         "nginx:1.25-alpine",
			Status:        "Ready",
			Age:           formatDuration(time.Duration(i) * time.Minute),
		}
	}

	b.ResetTimer()

	// Benchmark processing operations that our tool performs on deployment data
	for i := 0; i < b.N; i++ {
		// Simulate filtering deployments by status (common operation)
		readyCount := 0
		for _, deployment := range deployments {
			if deployment.Status == "Ready" {
				readyCount++
			}
		}

		// Simulate counting replicas across all deployments (common aggregation)
		totalReplicas := int32(0)
		for _, deployment := range deployments {
			totalReplicas += deployment.Replicas
		}

		// Use results to prevent optimization
		_ = readyCount
		_ = totalReplicas
	}
}

// BenchmarkPodInfoProcessing tests processing large pod datasets
// This simulates performance when dealing with clusters that have thousands of pods
func BenchmarkPodInfoProcessing(b *testing.B) {
	// Create realistic pod data simulating a busy production cluster
	pods := make([]PodInfo, 5000) // Typical large cluster might have thousands of pods

	for i := range pods {
		pods[i] = PodInfo{
			ClusterName: "benchmark-cluster",
			Namespace:   "production",
			Name:        generatePodName(i),
			Status:      selectPodStatus(i),
			Ready:       "1/1",
			Restarts:    int32(i % 3), // Simulate some pods with restarts
			Age:         formatDuration(time.Duration(i) * time.Second),
			Node:        generateNodeName(i % 10), // Simulate 10 nodes
			CreatedAt:   time.Now().Add(-time.Duration(i) * time.Second),
		}
	}

	b.ResetTimer()

	// Benchmark typical operations performed on pod data
	for i := 0; i < b.N; i++ {
		// Count pods by status (very common operation)
		statusCounts := make(map[string]int)
		for _, pod := range pods {
			statusCounts[pod.Status]++
		}

		// Find pods with high restart counts (troubleshooting operation)
		highRestartPods := 0
		for _, pod := range pods {
			if pod.Restarts > 5 {
				highRestartPods++
			}
		}

		// Group pods by node (capacity planning operation)
		nodeDistribution := make(map[string]int)
		for _, pod := range pods {
			nodeDistribution[pod.Node]++
		}

		// Use results to prevent optimization
		_ = statusCounts
		_ = highRestartPods
		_ = nodeDistribution
	}
}

// BenchmarkMemoryAllocation specifically tests memory allocation patterns
// This helps ensure our tool won't cause memory pressure in large environments
func BenchmarkMemoryAllocation(b *testing.B) {
	b.ReportAllocs() // Enable detailed allocation reporting

	for i := 0; i < b.N; i++ {
		// Simulate creating deployment info structures (what happens during cluster queries)
		deployment := DeploymentInfo{
			ClusterName:   "test-cluster",
			Namespace:     "default",
			Name:          "test-deployment",
			Replicas:      3,
			ReadyReplicas: 3,
			Image:         "nginx:latest",
			Status:        "Ready",
			Age:           "5m",
		}

		// Simulate appending to slices (what happens when collecting results from multiple clusters)
		deployments := make([]DeploymentInfo, 0, 100)
		for j := 0; j < 10; j++ {
			deployments = append(deployments, deployment)
		}

		// Use result to prevent optimization
		_ = deployments
	}
}

// Helper functions to generate realistic test data

func generateDeploymentName(index int) string {
	// Simulate realistic deployment names
	services := []string{"frontend", "backend", "api", "worker", "cache", "database"}
	return services[index%len(services)] + "-deployment"
}

func generatePodName(index int) string {
	// Simulate realistic pod names with random suffixes (as Kubernetes generates)
	services := []string{"frontend", "backend", "api", "worker", "cache"}
	service := services[index%len(services)]
	return service + "-pod-" + generateRandomSuffix(index)
}

func generateNodeName(index int) string {
	// Simulate realistic node names as cloud providers generate them
	return "node-" + generateRandomSuffix(index)
}

func generateRandomSuffix(seed int) string {
	// Simple deterministic "random" suffix for consistent benchmarking
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	result := ""
	for i := 0; i < 8; i++ {
		result += string(chars[(seed+i)%len(chars)])
	}
	return result
}

func selectPodStatus(index int) string {
	// Simulate realistic distribution of pod statuses
	// Most pods should be Running, with some in other states
	statuses := []string{
		"Running", "Running", "Running", "Running", "Running", // 50% running
		"Running", "Running", "Running", "Running", "Running",
		"Pending", "Pending", // 20% pending
		"Failed",    // 10% failed
		"Succeeded", // 10% succeeded
		"Unknown",   // 10% unknown
	}
	return statuses[index%len(statuses)]
}

// BenchmarkConcurrentProcessing tests performance under concurrent load
// This simulates what happens when your tool processes multiple clusters simultaneously
func BenchmarkConcurrentProcessing(b *testing.B) {
	// This benchmark would test goroutine performance, but we'll keep it simple for now
	// In a real-world tool, you'd want to benchmark the actual concurrent cluster operations

	testData := make([]string, 1000)
	for i := range testData {
		testData[i] = generateDeploymentName(i)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate concurrent string processing (simplified version of real operations)
			for _, name := range testData {
				if len(name) > 10 {
					_ = name[:10] // Simulate string manipulation
				}
			}
		}
	})
}
