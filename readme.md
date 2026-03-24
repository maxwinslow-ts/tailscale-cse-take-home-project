# Business Use Case: Secure Regional Connectivity PoC (🇺🇸/🇪🇺)

**Project:** Modernizing Legacy Infrastructure with Tailscale  
**Prepared By:** Max Winslow | **Date:** March 2026

<p align="center">
  <img src="img/us-screenshot.png" alt="US Dashboard" width="400" />
  <img src="img/eu-screenshot.png" alt="EU Dashboard" width="400" />
</p>

## 1. Project Overview (PoC)
**Goal:** Demonstrate secure, cross-region connectivity and "break-glass" access between two isolated legacy environments (US and EU) without manual VPN configuration.

* **Setup:** Two Ubuntu VMs (US and EU) each running a containerized Node.js application and a Tailscale Subnet Router.
* **Compliance Target:** Strict data residency and identity-based access logs.

---

## 2. The Problem: The "Manual Networking" Tax
The customer currently manages cross-region links and emergency access using manual `iptables`, `ip route` commands, and static SSH tunnels.

* **Operational Toil:** Without a control plane, cross-region connectivity requires manually configured tunnels with hardcoded peer addresses on each VM. When either VM is reprovisioned, both sides must be updated independently. Two machines, no mechanism to verify they're in sync.

* **Visibility "Blind Spots":** Subnet routers perform SNAT by default, rewriting container source IPs to the router's address. Cross-region traffic becomes unattributable to a specific container, failing SOC2 audit requirements.

* **Slow Emergency Access:** Static SSH keys carry no identity context. Revoking access during an incident means editing authorized_keys on each VM manually, with no audit trail of who used what key.

---

## 3. The Solution: Tailscale Identity Plane
We are replacing manual commands and static tunnels with a centralized Tailscale control plane. This provides one unified way to handle cross-region traffic and emergency access.

### Core Technical Plan

* **Site-to-Site Subnet Routing**
    * **The Fix:** A Tailscale [Subnet Router](https://tailscale.com/docs/features/site-to-site) container runs on each VM. It bridges the isolated Docker networks across regions.

    * **Benefit:** Replaces per-VM tunnel config with a managed mesh. Containerized services get cross-region connectivity without touching the host's underlying networking.

* **Audit-Ready Transparency (`--snat-subnet-routes=false`)**
    * **The Fix:** SNAT is disabled on each router, so the original container source IP is preserved end-to-end.

    * **Benefit:** Auditors can attribute every cross-region request to a specific container instance. This satisfies SOC2 traffic traceability requirements.

* **IdP-Backed SSH (Break-Glass)**
    * **The Fix:** [Tailscale SSH](https://tailscale.com/docs/features/ssh) is enabled on each VM host, gating access through the company's IdP instead of static keys.

    * **Benefit:** Port 22 can be closed. Access is MFA-protected, identity-scoped, and fully audited. No key distribution required.

---

## 4. Business Value (The "Why")
* **Zero Manual Sync:** Routing policy and ACLs are defined once in Terraform and propagated to both regions automatically. No per-VM edits, no opportunity for the two sides to drift.

* **Auditability by Default:** Preserved source IPs mean every cross-region request is traceable to a specific container instance, not just a router address. Compliance evidence is built into the data plane.

* **Faster Incident Response**: Break-glass access is a secure, audited login, not a key hunt. Engineers can reach any container in seconds, with a full identity trail.

* **Infrastructure Agnostic:** The same overlay works on local VMs, cloud instances, or bare metal. No changes to the underlying network required to connect isolated environments.

# Lab Setup

<p align="center">
  <img src="img/mermaid-diagram.png" alt="Architecture Diagram" width="700" />
</p>

<!-- BEGIN_TF_DOCS -->
## Requirements

- Terraform
- Tailscale
- OrbStack
- Mac OS
- Docker (on Mac, for pulling images)

## Providers

| Name | Version |
|------|---------|
| <a name="provider_local"></a> [local](#provider\_local) | 2.7.0 |
| <a name="provider_orbstack"></a> [orbstack](#provider\_orbstack) | 3.1.2 |
| <a name="provider_tailscale"></a> [tailscale](#provider\_tailscale) | 0.28.0 |

## Resources

| Name | Type |
|------|------|
| [local_file.compose](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [local_file.server_js](https://registry.terraform.io/providers/hashicorp/local/latest/docs/resources/file) | resource |
| [orbstack_machine.legacy_nodes](https://registry.terraform.io/providers/robertdebock/orbstack/latest/docs/resources/machine) | resource |
| [tailscale_acl.main_acl](https://registry.terraform.io/providers/tailscale/tailscale/latest/docs/resources/acl) | resource |
| [tailscale_tailnet_key.router](https://registry.terraform.io/providers/tailscale/tailscale/latest/docs/resources/tailnet_key) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_tailscale_admin_email"></a> [tailscale\_admin\_email](#input\_tailscale\_admin\_email) | Admin email for Tailscale ACL group | `string` | n/a | yes |
| <a name="input_tailscale_api_key"></a> [tailscale\_api\_key](#input\_tailscale\_api\_key) | Tailscale API Key for provider authentication | `string` | n/a | yes |
| <a name="input_tailscale_tailnet"></a> [tailscale\_tailnet](#input\_tailscale\_tailnet) | Tailscale Tailnet name for provider authentication | `string` | n/a | yes |

## Setup

First-time provisioning — creates VMs, pulls images, loads them, pushes files, and starts the stacks:

```sh
chmod +x ./cmd/*
./cmd/setup.sh
```

### Redeploy After Changes

Re-renders templates, pushes updated files, and restarts the stacks:

```sh
./cmd/redeploy.sh
```

### Teardown

Stops all containers and destroys the VMs:

```sh
./cmd/teardown.sh
```

---

## Verify Working Demo

```sh
./cmd/verify.sh
```

## Todos - Areas To Extend
- [ ] Add MySQL and a MySQL sidecar container on each VM connected via [Tailscale sidecar container](https://tailscale.com/blog/docker-tailscale-guide), with ACLs restricting access to only the Node.js app containers from both regions

- [ ] Systemd Tailscale service running on Host for IdP-based SSH access

- [ ] Deploy an Nginx reverse proxy on a separate Kubernetes cluster, routing to the US/EU Node.js apps via the tailnet using the [Kubernetes Operator](https://tailscale.com/docs/features/kubernetes-operator) — demonstrating the mesh extends to K8s workloads without reconfiguring existing VMs.




