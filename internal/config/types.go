package config

import "k8s.io/client-go/rest"

// ClusterConfig represents a single Kubernetes cluster configuration
// Think of this as a "business card" for each cluster - it tells us
// where the cluster is, how to connect to it, and what to call it
type ClusterConfig struct {
	Name        string `yaml:"name" json:"name"`                         // Human-readable name like "prod-us-east"
	Context     string `yaml:"context" json:"context"`                   // kubectl context name
	KubeConfig  string `yaml:"kubeconfig" json:"kubeconfig"`             // Path to kubeconfig file
	Region      string `yaml:"region,omitempty" json:"region"`           // Optional: AWS region, Azure location, etc.
	Environment string `yaml:"environment,omitempty" json:"environment"` // dev, staging, prod
	IsDefault   bool   `yaml:"default,omitempty" json:"default"`         // Mark one as default cluster
}

// MultiClusterConfig holds all our cluster configurations
// This is like a directory of all your clusters
type MultiClusterConfig struct {
	Clusters []ClusterConfig `yaml:"clusters" json:"clusters"`
	// Global settings that apply to all clusters
	DefaultNamespace string `yaml:"defaultNamespace,omitempty" json:"defaultNamespace"`
	Timeout          int    `yaml:"timeout,omitempty" json:"timeout"` // Connection timeout in seconds
}

// ClusterClient wraps the Kubernetes client with cluster metadata
// This combines the cluster info with an actual connection to that cluster
type ClusterClient struct {
	Config     ClusterConfig // The cluster configuration
	RestConfig *rest.Config  // Kubernetes REST client configuration
	Connected  bool          // Whether we successfully connected
	Error      error         // Any connection error
}
