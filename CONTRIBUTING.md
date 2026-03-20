# Contributing to OpenClaw Operator

Thank you for your interest in contributing!

## Development Setup

1. Fork and clone the repo
2. Install prerequisites: Go 1.25+, Docker, kubectl, operator-sdk
3. Create a Kind cluster: `kind create cluster --name openclaw-dev`
4. Install CRDs: `make install`
5. Run the controller locally: `make run`

## Making Changes

1. Create a branch: `git checkout -b feature/my-change`
2. Make your changes
3. Run linting: `make lint`
4. Run tests: `make test`
5. Verify build: `go build ./...`
6. Regenerate manifests if you changed types: `make generate manifests`

## Pull Requests

- Keep PRs focused on a single change
- Update tests for new functionality
- Update the README if you add user-facing features
- Run `make lint test` before pushing

## Code Style

- Follow standard Go conventions
- Use `controller-runtime` patterns (CreateOrUpdate, owner references, conditions)
- Keep reconciler methods small — one file per resource type

## Reporting Issues

Open a GitHub issue with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- Operator and Kubernetes versions
