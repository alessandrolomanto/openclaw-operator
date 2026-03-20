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

> **Warning**
> This project is under active development and is not yet considered production-ready.
> Use it at your own risk. The maintainers are not responsible for any issues arising
> from its use in production environments.

## Installation

### Option A: Helm (recommended)

```bash
helm install openclaw-operator \
  oci://ghcr.io/alessandrolomanto/openclaw-operator/charts/openclaw-operator \
  --namespace openclaw-operator-system \
  --create-namespace
```

Or from source:

```bash
helm install openclaw-operator \
  ./charts/openclaw-operator \
  --namespace openclaw-operator-system \
  --create-namespace
```

Custom values:

```bash
helm install openclaw-operator ./charts/openclaw-operator \
  --namespace openclaw-operator-system \
  --create-namespace \
  --set image.tag=v0.0.3 \
  --set resources.limits.memory=512Mi
```

### Option B: Kustomize

Using the standalone `kustomize` binary (required for remote refs — `kubectl -k` does not support them):

```bash
kustomize build 'github.com/alessandrolomanto/openclaw-operator?ref=v0.0.3' | kubectl apply -f -
```

Or from a local clone:

```bash
kubectl apply -k kustomize/
```

You can also create your own overlay referencing the repo as a remote base:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: my-namespace
resources:
  - github.com/alessandrolomanto/openclaw-operator?ref=v0.0.3
images:
  - name: controller
    newName: ghcr.io/alessandrolomanto/openclaw-operator
    newTag: v0.0.3
```

### Option C: From release manifest

```bash
kubectl apply -f https://github.com/alessandrolomanto/openclaw-operator/releases/download/v0.0.3/install.yaml
```

### Option D: CRD only

Install just the CRD (useful when running the operator outside the cluster):

```bash
kubectl apply -f https://raw.githubusercontent.com/alessandrolomanto/openclaw-operator/main/config/crd/bases/openclaw.nonnoalex.dev_openclawinstances.yaml
```

### Option E: From source (development)

```bash
make install    # install CRDs
make run        # run controller locally
```

## Quick Start

### 1. Create a Secret with your API keys

```bash
kubectl create secret generic openclaw-api-keys \
  --from-literal=ANTHROPIC_API_KEY=sk-your-key
```

### 2. Deploy an instance

```bash
kubectl apply -f config/samples/openclaw_v1alpha1_openclawinstance.yaml
```

### 3. Verify

```bash
kubectl get openclawinstance
# NAME       PHASE     READY   GATEWAY                        AGE
# my-agent   Running   True    my-agent.default.svc:18789     2m

kubectl get pods
kubectl get svc
```

### 4. Access the CLI

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

## Helm Values Reference

| Parameter | Default | Description |
|---|---|---|
| `replicaCount` | `1` | Operator replicas |
| `image.repository` | `ghcr.io/alessandrolomanto/openclaw-operator` | Operator image |
| `image.tag` | `(appVersion)` | Image tag |
| `resources.limits.cpu` | `200m` | CPU limit |
| `resources.limits.memory` | `256Mi` | Memory limit |
| `resources.requests.cpu` | `50m` | CPU request |
| `resources.requests.memory` | `128Mi` | Memory request |
| `leaderElection.enabled` | `true` | Enable leader election |
| `metrics.enabled` | `true` | Enable metrics endpoint |
| `metrics.port` | `8443` | Metrics port |
| `installCRDs` | `true` | Install CRDs with Helm |
| `sampleInstance.enabled` | `false` | Deploy a sample OpenClawInstance |

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
make docker-build IMG=ghcr.io/alessandrolomanto/openclaw-operator:dev
```

## Releasing

A single command bumps the version across the entire repository and regenerates all manifests:

```bash
make prepare-release NEW_VERSION=0.0.3
```

This updates:

| File | What changes |
|---|---|
| `Makefile` | `VERSION ?=` |
| `charts/openclaw-operator/Chart.yaml` | `version` + `appVersion` |
| `kustomize/kustomization.yaml` | `newTag` |
| `README.md` | All version references |
| `config/manager/kustomization.yaml` | Image tag (via kustomize) |
| `dist/install.yaml` | Regenerated install manifest |
| `bundle.yaml` | Regenerated bundle |

Then commit, tag, and push:

```bash
git add -A
git commit -m "release: v0.0.3"
git tag -a v0.0.3 -m "Release v0.0.3"
git push origin main --tags
```

Pushing the tag triggers the [release workflow](.github/workflows/release.yml) which builds the multi-arch image, pushes it to GHCR, packages the Helm chart, and creates a GitHub Release.

## License

Apache License 2.0
