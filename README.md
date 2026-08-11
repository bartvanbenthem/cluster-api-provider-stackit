# Cluster API provider for STACKIT

A [Cluster API](https://cluster-api.sigs.k8s.io/) infrastructure provider for
[STACKIT](https://www.stackit.de/), built on the official
[STACKIT Go SDK](https://github.com/stackitcloud/stackit-sdk-go). It provisions
self-managed Kubernetes clusters on STACKIT's IaaS compute service (`services/iaas`) —
the same role CAPD (Docker) or CAPO (OpenStack) play for their platforms: this
repository supplies the *infrastructure* CRDs and controllers, while the standard
upstream CAPI kubeadm bootstrap and control-plane providers handle the actual
Kubernetes bootstrapping.

## What it provisions

Given a `Cluster` + `StackitCluster` + `KubeadmControlPlane` +
`StackitMachineTemplate`(s) + `MachineDeployment`, the controllers in this repo:

- **`StackitCluster`** — creates (or adopts, if `spec.network.id` is set) an isolated
  network, a security group (SSH, Kubernetes API, and unrestricted intra-cluster
  traffic), and reserves a public IP for the control-plane endpoint.
- **`StackitMachine`** — creates a STACKIT server for each `Machine`, attaching the
  cluster's network/security group, waiting for the bootstrap data secret, and (for the
  first control-plane machine) attaching the cluster's reserved public IP.
- **`StackitClusterIdentity`** — points at a `Secret` holding STACKIT credentials
  (a service account key or bearer token), so different `StackitCluster`s can use
  different STACKIT projects/credentials.

CRDs: `StackitCluster`, `StackitMachine`, `StackitMachineTemplate`,
`StackitClusterIdentity` (all `infrastructure.cluster.x-k8s.io/v1alpha1`), implementing
the Cluster API `v1beta2` contract.

## Known limitations (v1alpha1)

- **Single control-plane endpoint, no managed load balancer.** The control-plane
  endpoint is a single reserved public IP attached to the first control-plane machine —
  it does not fail over automatically if that machine is replaced. For real HA,
  run [kube-vip](https://kube-vip.io/) as a static pod on the control-plane machines
  (the standard approach for bare-IaaS CAPI providers without a managed L4 LB), or
  set `spec.controlPlaneEndpoint` on the `StackitCluster` to point at your own
  externally managed load balancer instead of letting the controller allocate one.
  Wiring STACKIT's Load Balancer service (`services/loadbalancer` /
  `services/lbapplication` in the SDK) in as a first-class option is a natural
  follow-up.
- **No ClusterClass / `StackitClusterTemplate` support yet.**
- **No SSH key auto-provisioning.** `StackitMachine.spec.sshKeyName` must reference an
  existing STACKIT key pair; the controllers don't create one for you.
- **No webhooks.** Validation is enforced via CRD OpenAPI schema (required fields,
  string patterns) rather than admission webhooks, so there's no cert-manager
  dependency, but immutability rules (e.g. "you can't change a StackitMachine's
  imageId after creation") aren't enforced at the API level.

## Prerequisites

- Go 1.24+
- A STACKIT project and a [service account key](https://docs.stackit.cloud/stackit/en/service-accounts-devices-clients-and-keys-124321189.html)
  (or bearer token) with permissions to manage IaaS resources in that project
- A management cluster with [Cluster API core, kubeadm bootstrap, and kubeadm
  control-plane providers](https://cluster-api.sigs.k8s.io/user/quick-start) already
  installed, e.g. via:

  ```sh
  clusterctl init
  ```

- A STACKIT [image](https://docs.stackit.cloud/stackit/en/images-75989685.html) with a
  Kubernetes-ready OS (containerd, kubeadm/kubelet preinstalled, or use
  [image-builder](https://image-builder.sigs.k8s.io/) to build one) — note its ID.

## Building and deploying the manager

```sh
make manifests generate    # regenerate CRDs/RBAC and deepcopy code after any api/ change
make test                  # unit tests + envtest-backed controller tests
make docker-build docker-push IMG=<registry>/cluster-api-provider-stackit:<tag>
make install               # apply the CRDs to your management cluster
make deploy IMG=<registry>/cluster-api-provider-stackit:<tag>   # deploy the manager
```

`make manifests`/`make generate` intentionally scope `controller-gen` to `./api/...
./cmd/... ./internal/... ./test/...` — the vendored `stackit-sdk-go/` reference copy in
this repo is not part of this module and is excluded.

## Provisioning a cluster

1. Create the credentials `Secret` and `StackitClusterIdentity`:

   ```sh
   kubectl create secret generic my-cluster-credentials \
     --from-literal=serviceAccountKey="$(cat service-account-key.json)"

   kubectl apply -f - <<EOF
   apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
   kind: StackitClusterIdentity
   metadata:
     name: my-cluster-credentials
   spec:
     secretRef:
       name: my-cluster-credentials
   EOF
   ```

2. Render `templates/cluster-template.yaml` with the variables it needs (see the
   comment block at the top of that file for the full list) and apply it:

   ```sh
   export CLUSTER_NAME=my-cluster
   export NAMESPACE=default
   export STACKIT_PROJECT_ID=<your project id>
   export STACKIT_IMAGE_ID=<your image id>
   export STACKIT_SERVICE_ACCOUNT_KEY="$(cat service-account-key.json)"
   export KUBERNETES_VERSION=v1.31.0

   clusterctl generate yaml --from templates/cluster-template.yaml | kubectl apply -f -
   ```

   (`clusterctl generate yaml --from <file>` performs the same `${VAR}` substitution
   `clusterctl generate cluster` does, without requiring this provider to be published
   to a `clusterctl` provider repository yet. `envsubst < templates/cluster-template.yaml
   | kubectl apply -f -` works identically.)

3. Watch it come up:

   ```sh
   kubectl get cluster,stackitcluster,kubeadmcontrolplane,machines,stackitmachines
   clusterctl describe cluster my-cluster
   ```

`metadata.yaml` at the repo root is the `clusterctl` provider metadata file (contract
`v1beta2`); once this provider has a tagged GitHub release with CRD/manager manifests
attached, `clusterctl init --infrastructure stackit` can install it directly, and a
`clusterctl.yaml` provider entry can point at the release instead of local files.

## Repository layout

- `api/v1alpha1/` — CRD Go types (`StackitCluster`, `StackitMachine`,
  `StackitMachineTemplate`, `StackitClusterIdentity`)
- `internal/cloud/` — thin wrapper around `stackit-sdk-go/services/iaas/v2api`
  (networks, security groups, servers, public IPs, failure domains), built around the
  SDK's own `iaas.DefaultAPI` interface so it's mockable in tests
- `internal/controller/` — the four reconcilers
- `templates/cluster-template.yaml` — a full working cluster definition
  (`Cluster` + `StackitCluster` + `KubeadmControlPlane` + `StackitMachineTemplate` ×2 +
  `KubeadmConfigTemplate` + `MachineDeployment`)
- `config/` — kustomize manifests (CRDs, RBAC, manager Deployment)
- `stackit-sdk-go/` — a vendored reference copy of the STACKIT SDK for local browsing;
  this module actually depends on the published
  `github.com/stackitcloud/stackit-sdk-go` module via `go.mod`, not this directory
