terraform {
  required_providers {
    orbstack = {
      source = "robertdebock/orbstack"
    }
    tailscale = {
      source = "tailscale/tailscale"
    }
  }
}

provider "orbstack" {}

provider "tailscale" {
  api_key = var.tailscale_api_key
  tailnet = var.tailscale_tailnet
}