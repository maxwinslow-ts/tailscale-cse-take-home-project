variable "tailscale_admin_email" {
  description = "Admin email for Tailscale ACL group"
  type        = string
  sensitive   = true
}

variable "tailscale_api_key" {
  description = "Tailscale API Key for provider authentication"
  type        = string
  sensitive   = true
}

variable "tailscale_tailnet" {
  description = "Tailscale Tailnet name for provider authentication"
  type        = string
  sensitive   = true
}