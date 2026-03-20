---
name: Bug Report
about: Report a bug in the OpenClaw Operator
labels: bug
---

## Describe the bug

A clear description of what the bug is.

## To Reproduce

1. Apply this CR: `...`
2. Run `kubectl get openclawinstance`
3. See error

## Expected behavior

What you expected to happen.

## Environment

- Operator version: 
- Kubernetes version: `kubectl version`
- Platform: (Kind / EKS / GKE / AKS / other)

## Logs

```
kubectl logs -n openclaw-operator-system deploy/openclaw-operator-controller-manager
```
