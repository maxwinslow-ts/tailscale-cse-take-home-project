import {
  to = tailscale_acl.main_acl
  id = "acl"
}

# ── ACL Policy (Modern Grants) ───────────────────────────────────────
resource "tailscale_acl" "main_acl" {
  acl = jsonencode({

    # Tag ownership
    tagOwners = {
      "tag:database"   = [var.tailscale_admin_email],
      "tag:eu-router"  = [var.tailscale_admin_email],
      "tag:app-server" = [var.tailscale_admin_email],
    },

    # Access control (default-deny: only these grants allow traffic)
    grants = [
      {
        # US app containers → EU MySQL via eu-router subnet route.
        # With --snat-subnet-routes=false, source IP is the container's
        # real IP (172.21.0.x), so src must be the subnet CIDR, not a tag.
        src = ["${local.us_docker_cidr}"],
        dst = ["192.168.0.0/16"],
        ip  = ["tcp:3306"],
      },
      {
        # Admin full access for debugging & management
        src = ["autogroup:admin"],
        dst = ["*"],
        ip  = ["*"],
      },
    ],

    # Auto-approve subnet routes for designated tags
    autoApprovers = {
      routes = {
        "${local.us_docker_cidr}" = ["tag:app-server"],
        "192.168.0.0/16"          = ["tag:eu-router"],
      },
    },

    # Tailscale SSH: admin can SSH into all tagged nodes (IdP check)
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

# ── Auth Keys (one per role) ──────────────────────────────────────────
resource "tailscale_tailnet_key" "database" {
  reusable      = true
  ephemeral     = true
  preauthorized = true
  tags          = ["tag:database"]

  depends_on = [tailscale_acl.main_acl]
}

resource "tailscale_tailnet_key" "eu_router" {
  reusable      = true
  ephemeral     = true
  preauthorized = true
  tags          = ["tag:eu-router"]

  depends_on = [tailscale_acl.main_acl]
}

resource "tailscale_tailnet_key" "app_server" {
  reusable      = true
  ephemeral     = true
  preauthorized = true
  tags          = ["tag:app-server"]

  depends_on = [tailscale_acl.main_acl]
}