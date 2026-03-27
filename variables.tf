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

variable "mysql_root_password" {
  description = "Root password for the MySQL server on eu-db"
  type        = string
  sensitive   = true
  default     = "rootpass"
}

variable "mysql_database" {
  description = "Default database name created on eu-db"
  type        = string
  default     = "app"
}