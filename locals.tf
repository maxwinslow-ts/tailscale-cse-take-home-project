locals {
  # ── US Application Tier (Docker host) ───────────────
  us_docker_cidr = "172.21.0.0/24"
  us_router_ip   = cidrhost(local.us_docker_cidr, 2)  # 172.21.0.2 — Tailscale container
  us_app_ip      = cidrhost(local.us_docker_cidr, 10)  # 172.21.0.10 — European Viewer container

  # ── EU Database Tier ────────────────────────────────
  # eu-db bridge IP is dynamic (assigned by OrbStack at VM creation).
  # Retrieved via: orbstack_machine.eu_db.ip_address
}