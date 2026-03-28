# ── OrbStack VMs ─────────────────────────────────────────────────────
# Three Ubuntu VMs simulate geographically separated hosts.
# Ansible configures each after creation (see ansible/site.yml).

resource "orbstack_machine" "eu_db" {
  name  = "eu-db"      # EU MySQL server (bare-metal stand-in)
  image = "ubuntu:jammy"
}

resource "orbstack_machine" "eu_router" {
  name  = "eu-router"  # Tailscale subnet router fronting eu-db
  image = "ubuntu:jammy"
}

resource "orbstack_machine" "us_app" {
  name  = "us-app"     # US Docker host running the Node.js app + Tailscale sidecar
  image = "ubuntu:jammy"
}

# ── Outputs consumed by generate-inventory.sh ────────────────────────
# VM bridge IPs (OrbStack-assigned) and Tailscale auth keys are read by
# ansible/generate-inventory.sh to build the Ansible inventory.

output "eu_db_ip" {
  value = orbstack_machine.eu_db.ip_address
}

output "eu_router_ip" {
  value = orbstack_machine.eu_router.ip_address
}

output "us_app_ip" {
  value = orbstack_machine.us_app.ip_address
}

output "ts_key_database" {
  value     = tailscale_tailnet_key.database.key
  sensitive = true
}

output "ts_key_eu_router" {
  value     = tailscale_tailnet_key.eu_router.key
  sensitive = true
}

output "ts_key_app_server" {
  value     = tailscale_tailnet_key.app_server.key
  sensitive = true
}