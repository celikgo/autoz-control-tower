## docs/TROUBLESHOOTING.md
```markdown
# Troubleshooting Guide

## Common Issues

### Connection Problems
**Symptom**: "failed to connect to cluster"
**Solutions**:
1. Verify kubeconfig file exists and is readable
2. Check kubectl context: `kubectl config get-contexts`
3. Test direct connection: `kubectl --context=CONTEXT get nodes`
4. Verify network connectivity to cluster

### Authentication Errors
**Symptom**: "Unauthorized" or "Forbidden" errors
**Solutions**:
1. Check token expiration: `kubectl auth can-i get pods`
2. Verify RBAC permissions
3. Refresh credentials if using cloud provider auth

### Configuration Issues
**Symptom**: "cluster not found" or "invalid configuration"
**Solutions**:
1. Validate config: `mcm config validate`
2. Check config file location: `mcm config path`
3. Review cluster names and contexts in config file

## Debug Mode
Enable verbose logging:
```bash
mcm --verbose clusters list