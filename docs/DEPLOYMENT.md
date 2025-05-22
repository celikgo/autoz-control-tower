# Deployment Guide

## Container Deployment
```bash
# Build and run with Docker
docker build -t mcm:latest .
docker run --rm -it \
  -v ~/.kube:/root/.kube:ro \
  -v $(pwd)/configs:/app/configs:ro \
  mcm:latest clusters list