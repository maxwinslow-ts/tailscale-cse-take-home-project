import {
  to = tailscale_acl.main_acl
  id = "acl"
}

resource "tailscale_acl" "main_acl" {
  acl = jsonencode({
    tagOwners = {
      "tag:router" = [var.tailscale_admin_email],
    },

    groups = {
      "group:admin" = [var.tailscale_admin_email],
    },

    acls = [
      {
        action = "accept",
        src    = ["172.21.0.0/24"],
        dst    = ["172.22.0.0/24:*"],
      },
      {
        action = "accept",
        src    = ["172.22.0.0/24"],
        dst    = ["172.21.0.0/24:*"],
      },
      {
        action = "accept",
        src    = ["group:admin"],
        dst    = ["*:*"],
      },
    ],

    autoApprovers = {
      routes = {
        "172.21.0.0/24" = ["tag:router"],
        "172.22.0.0/24" = ["tag:router"],
      },
    },

    ssh = [
      {
        action = "check",
        src    = ["group:admin"],
        dst    = ["tag:router"],
        users  = ["root"],
      },
    ],
  })
}

resource "tailscale_tailnet_key" "router" {
  reusable      = true
  ephemeral     = true
  preauthorized = true
  tags          = ["tag:router"]

  depends_on = [tailscale_acl.main_acl]
}