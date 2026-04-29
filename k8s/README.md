# Kubernetes Quickstart

## Cluster Size

Use a three-node kind cluster for the interview demo:

```text
1 control-plane + 2 workers
```

One node is enough to verify `Deployment`, `Service`, and `ConfigMap`, but three nodes make the scheduling and load-balancing story clearer. The executor has three replicas, so you can show how Kubernetes schedules pods across nodes while the server only talks to the stable executor Service DNS name.

## Create A Kind Cluster

```bash
make kind-create
```

Equivalent command:

```bash
kind create cluster --name mini-llm --config k8s/kind/cluster.yaml
```

## Load Local Images

Kind nodes do not automatically see images from the host Docker daemon. Load the prepared local images into the cluster:

```bash
make kind-load-images
```

This loads:

```text
mini-llm-server:local
mini-llm-executor:local
```

## Deploy

Render first if you want to inspect the final YAML:

```bash
make k8s-render
```

Apply the base deployment:

```bash
make k8s-apply
make k8s-rollout
make k8s-status
```

## Optional: Ingress

Install ingress-nginx if the cluster does not already have an Ingress controller:

```bash
make k8s-install-ingress-nginx
```

Add a local hosts entry so `mini-llm.local` resolves to your machine:

```text
127.0.0.1 mini-llm.local
```

Then apply the app manifests and test:

```bash
make k8s-apply
curl http://mini-llm.local/metrics
```

Ingress exposes only the server Service. The executor Service stays internal because it is part of the execution plane, not the public request ingress.

## Verify Metrics

Forward the admin/metrics port:

```bash
make k8s-port-forward-admin
```

In another shell:

```bash
curl http://127.0.0.1:8801/metrics
```

## Request Flow

```text
client
  -> demo-server Service
  -> demo-server Pod
  -> executorManager logical executor slot
  -> http://demo-executor:19991
  -> demo-executor Service
  -> one executor Pod
```

The server config intentionally creates three logical executor slots, all pointing to the same Kubernetes Service:

```toml
address = ["http://demo-executor:19991"]
```

The current application controls batch concurrency through logical executor slots. Kubernetes controls pod selection through the Service.
