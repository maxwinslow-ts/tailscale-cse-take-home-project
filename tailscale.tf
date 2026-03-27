# Import the existing tailnet ACL so Terraform manages it in-place
# rather than failing on a conflict with the pre-existing policy.
import {
  to = tailscale_acl.main_acl
  id = "acl"
}

# ── ACL Policy ───────────────────────────────────────────────────────
# Default-deny posture. Only the grants below permit traffic.
resource "tailscale_acl" "main_acl" {
  acl = jsonencode({

    # Admin email owns all tags (required before tags can be assigned)
    tagOwners = {
      "tag:database"   = [var.tailscale_admin_email],
      "tag:eu-router"  = [var.tailscale_admin_email],
      "tag:app-server" = [var.tailscale_admin_email],
    },

    grants = [
      {
        # US Docker containers → EU MySQL (tcp:3306 only).
        # Source is the Docker bridge CIDR because SNAT is disabled on
        # the subnet router, so packets arrive with their real container IP.
        src = ["${local.us_docker_cidr}"],
        dst = ["192.168.0.0/16"],
        ip  = ["tcp:3306"],
      },
      {
        # Admin: unrestricted access for debugging
        src = ["autogroup:admin"],
        dst = ["*"],
        ip  = ["*"],
      },
    ],

    # Auto-approve advertised subnet routes by tag
    autoApprovers = {
      routes = {
        "${local.us_docker_cidr}" = ["tag:app-server"],
        "192.168.0.0/16"          = ["tag:eu-router"],
      },
    },

    # Tailscale SSH with IdP identity check for audit trail
    ssh = [
      {
        action = "check",
        src    = ["autogroup:admin"],
        dst    = ["tag:database", "tag:eu-router", "tag:app-server"],
        users  = ["root"],
      },
    ],
  })
}

# ── Auth Keys ────────────────────────────────────────────────────────
# One pre-authorized, ephemeral key per role. Each key is tagged so the
# node inherits ACL permissions at join time. Keys depend on the ACL to
# ensure tags exist before they are referenced.

resource "tailscale_tailnet_key" "database" {
  reusable      = true
  ephemeral     = true
  preauthorized = true
  tags          = ["tag:database"]
  depends_on    = [tailscale_acl.main_acl]
}

resource "tailscale_tailnet_key" "eu_router" {
  reusable      = true
  ephemeral     = true
  preauthorized = true
  tags          = ["tag:eu-router"]
  depends_on    = [tailscale_acl.main_acl]
}

resource "tailscale_tailnet_key" "app_server" {
  reusable      = true
  ephemeral     = true
  preauthorized = true
  tags          = ["tag:app-server"]
  depends_on    = [tailscale_acl.main_acl]
}