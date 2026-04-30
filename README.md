# kurt — Kubernetes Unified Resource Tree
CLI tool for visualizing Kubernetes resource relationship trees.

## Overview
kurt visualizes Kubernetes resource relationship trees by traversing parent-child and network connections. Starting from a Deployment, StatefulSet, Service, Ingress, Istio VirtualService, or Istio AuthorizationPolicy, it follows ownerReferences, Service label selectors, Ingress backends, VirtualService route destinations, Gateway references, AuthorizationPolicy selectors, and DestinationRule host references.

## Features
- Deployment tree: Deployment → ReplicaSets → Pods, plus matching Services → VirtualServices → Gateways + DestinationRules + AuthorizationPolicies
- StatefulSet tree: StatefulSet → Pods, plus matching Services → VirtualServices → Gateways + DestinationRules + AuthorizationPolicies
- VirtualService tree (reverse direction): VirtualService → Gateways + destination Services (with DestinationRules) → Deployments/StatefulSets (with AuthorizationPolicies) → ReplicaSets → Pods
- Service tree: Service → VirtualServices → Gateways + DestinationRules + Deployments (with AuthorizationPolicies) → ReplicaSets → Pods + StatefulSets (with AuthorizationPolicies) → Pods
- Ingress tree: Ingress → backend Services → VirtualServices → Gateways + DestinationRules + Deployments (with AuthorizationPolicies) → ReplicaSets → Pods + StatefulSets (with AuthorizationPolicies) → Pods
- AuthorizationPolicy tree: AuthorizationPolicy → matched Deployments → ReplicaSets → Pods + matched StatefulSets → Pods
- All overview: Scans all Deployments and StatefulSets in the namespace and renders a tree for each
- Color-coded output: green for healthy (Running, `3/3`, mTLS MUTUAL), red for errors (Failed, CrashLoopBackOff, `1/3`, mTLS DISABLE), yellow for transitional (Pending, mTLS PERMISSIVE), dimmed for inactive ReplicaSets (when shown)
- `--no-color` flag for pipe-friendly plain text output
- Pod readiness shown as container counts (`1/2`, `3/3`) matching kubectl style
- Multiple resources in a single command: `kurt deploy app-a app-b`
- kubectl-style columnar table with tree-drawing characters
- Automatic namespace detection from current kubeconfig context
- Istio CRD support is optional, gracefully skipped when CRDs are not installed
- Shell completion for bash, zsh, fish, and powershell
- `--exclude` flag to hide specific resource kinds from the tree (e.g. `--exclude replicaset,virtualservice`)
- `--hosts` flag to show VirtualService and Ingress hosts as an additional HOSTS column
- Interactive resource selection with [fzf](https://github.com/junegunn/fzf) when no resource name is given (falls back to standard error if fzf is not installed)
- `--watch` / `-w` flag for continuous re-rendering of the tree at a fixed interval (default 5s), with configurable `--interval` duration
- Inactive (zero-pod) ReplicaSets are hidden by default for a cleaner tree; use `--show-inactive` to include them

## Requirements
- Go 1.26+ (when building from source)
- Access to a Kubernetes cluster
## Installation
Install via Homebrew:

```bash
brew install thecheerfuldev/cli/kurt
```

## Build from source
Build from source with the provided Makefile:

```bash
make build
```

The binary is saved to `bin/kurt`.

## Usage
kurt supports several subcommands and global configuration flags. Each resource subcommand accepts one or more resource names.

### Subcommands
- `kurt deployment <name...>` (alias: `deploy`)
- `kurt statefulset <name...>` (alias: `sts`)
- `kurt virtualservice <name...>` (alias: `vs`)
- `kurt service <name...>` (alias: `svc`)
- `kurt ingress <name...>` (alias: `ing`)
- `kurt authorizationpolicy <name...>` (alias: `ap`)
- `kurt all` — show trees for all Deployments and StatefulSets in the namespace
- `kurt completion [bash|zsh|fish|powershell]` — generate shell completion scripts

### Global Flags
- `--kubeconfig`: Path to the kubeconfig file
- `--context`: Name of the kubeconfig context
- `-n, --namespace`: Namespace for the resource
- `--no-color`: Disable color output (also useful when piping)
- `--exclude`: Comma-separated list of resource kinds to exclude from the tree (e.g. `replicaset,virtualservice`)
- `--hosts`: Show a HOSTS column with VirtualService and Ingress hostnames
- `-w, --watch`: Continuously re-render the tree at a fixed interval
- `--interval`: Refresh interval for `--watch` (default `5s`, e.g. `1s`, `10s`, `500ms`)
- `--show-inactive`: Show inactive (zero-pod) ReplicaSets that are hidden by default

### Example
```bash
kurt deploy my-app -n production
kurt svc my-service my-other-service
kurt vs my-virtualservice --no-color
kurt ing my-ingress
kurt all -n default
kurt ap allow-to-excalidraw -n production
kurt deploy my-app --exclude replicaset,virtualservice
kurt deploy my-app --hosts
kurt deploy my-app --show-inactive                  # include old ReplicaSets with no pods
kurt ing my-ingress --hosts
kurt deploy                                        # interactive fzf picker
kurt svc -n kube-system                             # fzf picks a service in kube-system
kurt deploy my-app -w                              # watch mode, refreshes every 5s
kurt deploy my-app --watch --interval 10s           # watch mode, refreshes every 10s
kurt all -n production -w                           # watch all trees in a namespace
```

## Example Output
```
NAMESPACE      NAME                                              READY  REASON  STATUS   AGE
p-handy-tools  Deployment/excalidraw                             -              -        78d
p-handy-tools  ├─ReplicaSet/excalidraw-6cc8444659                -              -        78d
p-handy-tools  │ └─Pod/excalidraw-6cc8444659-qgks9               3/3            Running  43d
p-handy-tools  ├─Service/excalidraw-svc                          -              -        78d
p-handy-tools  │ ├─VirtualService/excalidraw-vs                  -              -        78d
p-handy-tools  │ │ └─Gateway/gui-gateway                        -              -        78d
p-handy-tools  │ └─DestinationRule/excalidraw-dr                 -              MUTUAL   78d
p-handy-tools  ├─AuthorizationPolicy/allow-to-excalidraw         -              ALLOW    78d
p-handy-tools  └─AuthorizationPolicy/allow-project-team          -              ALLOW    78d
```

Inactive (zero-pod) ReplicaSets are hidden by default for a cleaner view. Use `--show-inactive` to include them — they are displayed in dimmed text. Status columns are color-coded in terminal output.

### Example Output with `--hosts`
```
NAMESPACE      NAME                                              READY  REASON  STATUS   HOSTS                    AGE
p-handy-tools  Deployment/excalidraw                             -              -                                 78d
p-handy-tools  ├─ReplicaSet/excalidraw-6cc8444659                -              -                                 78d
p-handy-tools  │ └─Pod/excalidraw-6cc8444659-qgks9               3/3            Running                           43d
p-handy-tools  ├─Service/excalidraw-svc                          -              -                                 78d
p-handy-tools  │ ├─VirtualService/excalidraw-vs                  -              -        excalidraw.example.com   78d
p-handy-tools  │ │ └─Gateway/gui-gateway                        -              -                                 78d
p-handy-tools  │ └─DestinationRule/excalidraw-dr                 -              MUTUAL                            78d
p-handy-tools  ├─AuthorizationPolicy/allow-to-excalidraw         -              ALLOW                             78d
p-handy-tools  └─AuthorizationPolicy/allow-project-team          -              ALLOW                             78d
```

The HOSTS column is only shown when `--hosts` is passed. It displays the `spec.hosts` of VirtualServices and `spec.rules[].host` of Ingress resources.

## How It Works
- Deployment and StatefulSet resources find owned ReplicaSets and Pods via metadata.ownerReferences.
- Services are matched via label selector against pod template labels.
- VirtualServices are discovered by matching route destination hosts to Service names.
- Gateways are resolved from VirtualService spec.gateways references.
- The VirtualService subcommand reverses the direction, traversing from the VirtualService to Services and then to workloads.
- The Service subcommand shows VirtualServices above and Deployments/StatefulSets below the Service.
- The Ingress subcommand follows backend service references and then traverses each service's workloads.
- The `all` subcommand lists every Deployment and StatefulSet in the namespace and builds a tree for each.
- DestinationRules are matched by `spec.host` against Service names (short name, namespace-qualified, or FQDN) and shown as children of Service nodes.
- The mTLS mode from `spec.trafficPolicy.tls.mode` is displayed in the STATUS column with color coding: green for MUTUAL/ISTIO_MUTUAL, red for DISABLE, yellow for PERMISSIVE/SIMPLE.
- AuthorizationPolicies are matched via `spec.selector.matchLabels` against pod template labels; policies with no selector are namespace-wide and match all workloads.
- The AuthorizationPolicy subcommand reverses the direction, finding all Deployments and StatefulSets whose pod template labels match the policy's selector.

## Shell Completion

Generate and load shell completions:

```bash
# Bash
source <(kurt completion bash)

# Zsh
kurt completion zsh > "${fpath[1]}/_kurt"

# Fish
kurt completion fish | source

# PowerShell
kurt completion powershell | Out-String | Invoke-Expression
```

## Interactive Mode (fzf)

When you run a resource subcommand **without specifying a name** and [fzf](https://github.com/junegunn/fzf) is installed, kurt automatically opens an interactive fuzzy-finder to let you pick a resource from the cluster.

```bash
# No name given — fzf opens with all deployments in the current namespace
kurt deploy

# Works with any subcommand and flags
kurt svc -n production
kurt vs --hosts
```

If fzf is not installed or stdin is not a terminal (e.g. piped input), kurt prints an error asking for at least one resource name. Tab-completion continues to work independently of fzf.

## Watch Mode

Use `--watch` (or `-w`) to continuously re-render the tree at a fixed interval. The display clears and refreshes automatically, similar to the `watch` command.

```bash
# Refresh every 5 seconds (default)
kurt deploy my-app -w

# Custom interval
kurt deploy my-app --watch --interval 10s

# Watch all trees in a namespace
kurt all -n production -w
```

A dim timestamp header is shown at the top of each refresh. Press `Ctrl+C` to stop watching.
