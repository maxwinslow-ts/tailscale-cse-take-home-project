# ── EU Database Server ────────────────────────────────────────────────
resource "orbstack_machine" "eu_db" {
  name  = "eu-db"
  image = "ubuntu:jammy"
}

# ── EU Subnet Router ─────────────────────────────────────────────────
resource "orbstack_machine" "eu_router" {
  name  = "eu-router"
  image = "ubuntu:jammy"
}

# ── US Application Server ────────────────────────────────────────────
resource "orbstack_machine" "us_app" {
  name  = "us-app"
  image = "ubuntu:jammy"
}

# ── Outputs for Ansible inventory ────────────────────────────────────
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