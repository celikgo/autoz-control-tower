# Multi-Cluster Manager (MCM)

A powerful CLI tool for managing Kubernetes workloads across multiple clusters simultaneously. Built with Go and designed for platform engineers who need to operate applications across multiple environments, regions, or cloud providers.

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](#building)

## 🚀 Features

### Multi-Cluster Operations
- **Parallel Cluster Management**: Connect to and manage multiple Kubernetes clusters simultaneously
- **Unified Dashboard**: View deployments, pods, and services across all your clusters in one place
- **Cross-Cluster Deployments**: Deploy applications to multiple clusters in parallel with detailed progress tracking
- **Environment Awareness**: Organize clusters by environment (dev, staging, prod) and region

### Developer Experience
- **kubectl-Like Interface**: Familiar command structure for Kubernetes practitioners
- **Multiple Output Formats**: Table, JSON, and YAML output for both human and machine consumption
- **Rich Error Handling**: Detailed error messages and troubleshooting guidance
- **Configuration Validation**: Built-in validation for cluster configurations and connectivity

### Enterprise Ready
- **Robust Connection Management**: Automatic connection pooling and retry logic
- **Security First**: Secure handling of kubeconfig files and credentials
- **Observability**: Comprehensive logging and error reporting
- **Cross-Platform**: Support for Linux, macOS, and Windows

## 🏗️ Architecture

MCM follows a modular architecture designed for reliability and extensibility:

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   CLI Layer     │    │  Workload Mgmt   │    │  Cluster Mgmt   │
│                 │    │                  │    │                 │
│ • Command Parsing│◄──►│ • Deployments   │◄──►│ • Connections   │
│ • Output Format │    │ • Pod Management │    │ • Auth Handling │
│ • User Feedback │    │ • Multi-Cluster  │    │ • Health Checks │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                        │                        │
         └────────────────────────┼────────────────────────┘
                                  │
                    ┌─────────────▼──────────────┐
                    │     Configuration Mgmt     │
                    │                            │
                    │ • YAML Config Loading      │
                    │ • Kubeconfig Integration   │
                    │ • Validation & Defaults    │
                    └────────────────────────────┘
```

## 🛠️ Installation

### From Source
```bash
# Clone the repository
git clone https://github.com/celikgo/autoz-control-tower.git
cd multicluster-manager

# Build and install
make install

# Or just build locally
make build
./build/mcm --help
```

### Using Go Install
```bash
go install github.com/celikgo/autoz-control-tower/cmd/mcm@latest
```

### Using Docker
```bash
docker pull ghcr.io/celikgo/autoz-control-tower:latest

# Run with your kubeconfig mounted
docker run --rm -it \
  -v ~/.kube:/root/.kube:ro \
  -v $(pwd)/configs:/app/configs:ro \
  ghcr.io/celikgo/autoz-control-tower:latest
```

### Pre-built Binaries
Download the latest release from the [releases page](https://github.com/celikgo/autoz-control-tower/releases).

## 🚀 Quick Start

### 1. Initialize Configuration
```bash
# Create a sample configuration file
mcm config init

# Edit the configuration to match your clusters
vim ~/.config/mcm/config.yaml
```

### 2. Verify Cluster Connectivity
```bash
# Test connections to all configured clusters
mcm clusters list

# Validate configuration and connectivity
mcm config validate
```

### 3. Explore Your Infrastructure
```bash
# View all deployments across all clusters
mcm deployments list

# View deployments in specific clusters
mcm deployments list --clusters=prod-us,prod-eu

# Check pod status across all clusters
mcm pods list --namespace=default
```

### 4. Deploy Applications
```bash
# Deploy to specific clusters
mcm deploy app.yaml --clusters=prod-us,prod-eu

# Deploy to all production clusters
mcm deploy app.yaml --all-clusters --exclude=dev-cluster
```

## 📋 Configuration

MCM uses a YAML configuration file to define your cluster landscape:

```yaml
# ~/.config/mcm/config.yaml
defaultNamespace: "default"
timeout: 30

clusters:
  - name: "dev-cluster"
    context: "dev-context"
    kubeconfig: "~/.kube/config"
    environment: "development"
    region: "us-west-2"
    default: true

  - name: "prod-us-east"
    context: "production-us-east"
    kubeconfig: "~/.kube/prod-config"
    environment: "production"
    region: "us-east-1"

  - name: "prod-eu-west"
    context: "production-eu-west"
    kubeconfig: "~/.kube/prod-config"
    environment: "production"
    region: "eu-west-1"
```

### Configuration Locations
MCM looks for configuration files in this order:
1. `./mcm-config.yaml` (current directory)
2. `~/.mcm/config.yaml` (user home directory)
3. `$XDG_CONFIG_HOME/mcm/config.yaml` (XDG config directory)

## 📚 Usage Examples

### Cluster Management
```bash
# List all configured clusters with their status
mcm clusters list

# Test connectivity to all clusters
mcm clusters test

# Show cluster information in JSON format
mcm clusters list --output=json
```

### Deployment Operations
```bash
# List all deployments across all clusters
mcm deployments list

# Filter by specific clusters
mcm deployments list --clusters=prod-us,staging

# Filter by namespace
mcm deployments list --namespace=kube-system

# Export deployment info as JSON for further processing
mcm deployments list --output=json | jq '.deployments[] | select(.status=="NotReady")'
```

### Pod Management
```bash
# List all pods across all clusters
mcm pods list

# Filter by label selector
mcm pods list --selector="app=nginx,tier=frontend"

# View pods in specific namespace and clusters
mcm pods list --namespace=production --clusters=prod-us,prod-eu
```

### Multi-Cluster Deployments
```bash
# Deploy application to specific clusters
mcm deploy app.yaml --clusters=prod-us,prod-eu --namespace=production

# Deploy to all clusters except development
mcm deploy app.yaml --all-clusters --exclude=dev-cluster

# Deploy with custom namespace
mcm deploy app.yaml --clusters=staging --namespace=testing
```

## 🔧 Development

### Prerequisites
- Go 1.21 or later
- Access to one or more Kubernetes clusters
- kubectl configured with appropriate contexts

### Setting Up Development Environment
```bash
# Clone and setup
git clone https://github.com/celikgo/autoz-control-tower.git
cd multicluster-manager

# Install development dependencies
make dev-setup

# Run the development workflow
make dev
```

### Building and Testing
```bash
# Format code and run tests
make check

# Build for current platform
make build

# Build for all platforms
make build-all

# Run with race detection and coverage
make test-coverage

# Create a release
make release
```

### Project Structure
```
multicluster-manager/
├── cmd/mcm/                 # CLI entry point and command definitions
├── internal/
│   ├── cluster/            # Cluster connection management
│   ├── config/             # Configuration loading and validation
│   └── workload/           # Workload operations (deployments, pods)
├── configs/                # Sample configuration files
├── docs/                   # Documentation
├── build/                  # Build outputs
└── dist/                   # Distribution packages
```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

### Development Workflow
1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (`make check`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Code Standards
- Follow Go best practices and idioms
- Add tests for new functionality
- Update documentation for user-facing changes
- Use conventional commit messages

## 📖 Use Cases

### DevOps and Platform Engineering
- **Multi-Region Deployments**: Deploy applications across multiple geographic regions for high availability
- **Environment Parity**: Ensure consistency between development, staging, and production environments
- **Infrastructure Migration**: Gradually migrate workloads between clusters or cloud providers
- **Disaster Recovery**: Quickly deploy applications to backup clusters during incidents

### Development Teams
- **Cross-Environment Debugging**: Compare application state across different environments
- **Staged Rollouts**: Deploy to development and staging environments before production
- **Feature Testing**: Deploy feature branches to dedicated testing clusters

### Operations Teams
- **Health Monitoring**: Monitor application health across entire infrastructure
- **Capacity Planning**: Understand resource usage patterns across clusters
- **Incident Response**: Quickly assess and respond to issues across multiple environments

## 🛡️ Security Considerations

- **Kubeconfig Security**: MCM respects standard kubeconfig permissions and never modifies authentication files
- **Credential Isolation**: Each cluster connection uses isolated credentials
- **Network Security**: All cluster communications use the same security channels as kubectl
- **Audit Trail**: All operations are logged for security auditing

## 📊 Performance

MCM is designed for performance with large-scale multi-cluster environments:

- **Parallel Operations**: All cluster operations run in parallel, not sequentially
- **Connection Pooling**: Maintains persistent connections to reduce latency
- **Efficient Querying**: Optimized Kubernetes API usage to minimize network overhead
- **Graceful Degradation**: Continues operating even when some clusters are unavailable

## 🐛 Troubleshooting

### Common Issues

**Cluster Connection Failures**
```bash
# Validate your configuration
mcm config validate

# Test individual cluster connectivity
kubectl --context=your-context get nodes

# Check kubeconfig file permissions
ls -la ~/.kube/config
```

**Permission Denied Errors**
```bash
# Verify you have appropriate RBAC permissions
kubectl auth can-i create deployments --namespace=default

# Check your kubeconfig context
kubectl config current-context
```

**Configuration Issues**
```bash
# Show current configuration
mcm config show

# Validate configuration syntax
mcm config validate

# Show configuration file location
mcm config path
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI framework
- Uses [client-go](https://github.com/kubernetes/client-go) for Kubernetes API interactions
- Inspired by kubectl and other Kubernetes tooling
- Thanks to the Kubernetes community for excellent documentation and examples

---

**Built with ❤️ for the Kubernetes community**