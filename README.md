# OpenClaw Operator

Kubernetes operator for deploying and managing [OpenClaw](https://openclaw.ai/) AI agent instances.

A single `OpenClawInstance` custom resource reconciles into a fully managed stack:
ConfigMap, PVCs, StatefulSets, Services — with config merge, tool injection, and Ollama integration.

## Features

- **Declarative deployment** — one CR defines the entire stack
- **Config merge mode** — agents modify `openclaw.json` at runtime; the operator deep-merges base config without overwriting agent changes
- **Tool injection** — install CLI tools (`jq`, `gh`, `helm`, `kubectl`, etc.) via `apt-get` in an init container, no custom image needed
- **Ollama as a separate StatefulSet** — dedicated deployment with its own PVC for model persistence
- **CLI sidecar** — interactive TUI access via `kubectl exec`
- **Self-healing** — owner references + drift detection every 5 minutes
- **Readiness-aware status** — phase reflects actual pod readiness, not just resource creation

## Quick Start

### Prerequisites

- Kubernetes 1.28+
- kubectl configured with cluster access

### 1. Install the CRD and operator

```bash
# From source
make install   # install CRDs
make run       # run controller locally
```

### 2. Create a Secret with your API keys

```bash
kubectl create secret generic openclaw-api-keys \
  --from-literal=ANTHROPIC_API_KEY=sk-your-key
```

### 3. Deploy an instance

```bash
kubectl apply -f config/samples/openclaw_v1alpha1_openclawinstance.yaml
```

### 4. Verify

```bash
kubectl get openclawinstance
# NAME       PHASE     READY   GATEWAY                        AGE
# my-agent   Running   True    my-agent.default.svc:18789     2m

kubectl get pods
kubectl get svc
```

### 5. Access the CLI

```bash
kubectl exec -it my-agent-0 -c cli -- node /app/dist/index.js
```

## Example CR

```yaml
apiVersion: openclaw.nonnoalex.dev/v1alpha1
kind: OpenClawInstance
metadata:
  name: my-agent
spec:
  image:
    repository: ghcr.io/openclaw/openclaw
    tag: "latest"

  config:
    mergeMode: merge
    raw:
      gateway:
        port: 18789
        mode: local
        bind: lan

  envFrom:
    - secretRef:
        name: openclaw-api-keys

  tools:
    - jq
    - vim
    - curl

  resources:
    requests:
      cpu: "500m"
      memory: "1Gi"
    limits:
      cpu: "2"
      memory: "4Gi"

  storage:
    size: "20Gi"

  ollama:
    enabled: true
    storage:
      size: "50Gi"

  cli:
    enabled: true
```

## Architecture

```
OpenClawInstance CR
       │ watch
       ▼
 Operator (controller-runtime)
       │ reconciles in order
       ▼
 1. ConfigMap         (openclaw.json base config)
 2. PVC               (gateway data — config + workspace)
 3. Ollama PVC        (model storage)
 4. Ollama STS        (separate StatefulSet)
 5. Ollama Service    (ClusterIP on 11434)
 6. Gateway STS       (with init containers + CLI sidecar)
       ├── init-config   (seed/merge config from ConfigMap to PVC)
       ├── init-tools    (apt-get install + copy binaries & libs)
       ├── openclaw      (gateway process)
       └── cli           (interactive TUI sidecar)
 7. Gateway Service   (ClusterIP on 18789, 18790)
```

## CRD Reference

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.image` | `ImageSpec` | `ghcr.io/openclaw/openclaw:latest` | Container image |
| `spec.config` | `ConfigSpec` | — | OpenClaw config (inline or ConfigMap ref) |
| `spec.config.mergeMode` | `string` | `merge` | `merge` preserves runtime changes, `overwrite` replaces |
| `spec.envFrom` | `[]EnvFromSource` | — | Inject env vars from Secrets |
| `spec.env` | `[]EnvVar` | — | Individual env vars |
| `spec.resources` | `ResourceRequirements` | — | CPU/memory for gateway |
| `spec.storage.size` | `string` | `10Gi` | Data PVC size |
| `spec.tools` | `[]string` | — | Tools to install via apt-get |
| `spec.ollama.enabled` | `bool` | `false` | Deploy Ollama |
| `spec.ollama.storage.size` | `string` | `50Gi` | Ollama models PVC size |
| `spec.cli.enabled` | `bool` | `false` | Add CLI sidecar |

## Development

```bash
# Generate deepcopy and CRD manifests
make generate
make manifests

# Build
go build ./...

# Run tests
make test

# Run locally against a cluster
make install
make run

# Build Docker image
make docker-build IMG=ghcr.io/your-org/openclaw-operator:v0.0.1
```

## License

Apache License 2.0
