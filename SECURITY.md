# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.0.x   | Yes       |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do not** open a public GitHub issue
2. Email security concerns to the maintainer directly
3. Include a description of the vulnerability, steps to reproduce, and potential impact

We will acknowledge receipt within 48 hours and provide a timeline for a fix.

## Security Measures

The operator follows these security practices:

- **Non-root containers**: Manager runs as UID 65532 (distroless nonroot)
- **Read-only root filesystem**: Distroless base image
- **Dropped capabilities**: All Linux capabilities dropped
- **Seccomp**: RuntimeDefault profile on pod and container level
- **Minimal RBAC**: Only the permissions needed for reconciliation
- **No privilege escalation**: `allowPrivilegeEscalation: false`
- **Dependency scanning**: Trivy and gosec run in CI
