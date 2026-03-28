# ── Tailscale provider & ACL inputs ──────────────────────────────────

variable "tailscale_admin_email" {
  description = "Email tied to ACL tag ownership (appears in tagOwners)"
  type        = string
  sensitive   = true
}

variable "tailscale_api_key" {
  description = "API key used by the Tailscale Terraform provider"
  type        = string
  sensitive   = true
}

variable "tailscale_tailnet" {
  description = "Tailnet org name used by the Tailscale Terraform provider"
  type        = string
  sensitive   = true
}

# ── MySQL inputs (passed through to Ansible via inventory) ───────────

variable "mysql_root_password" {
  description = "Root password for the MySQL instance on eu-db"
  type        = string
  sensitive   = true
  default     = "rootpass"
}

variable "mysql_database" {
  description = "Default database created on eu-db at provision time"
  type        = string
  default     = "app"
}