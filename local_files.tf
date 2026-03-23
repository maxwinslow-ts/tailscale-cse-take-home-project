resource "local_file" "compose" {
  for_each = local.regions

  content = templatefile("${path.module}/orb-templates/bootstrap-compose.yaml.tftpl", {
    region_name = each.key
    cidr        = each.value.cidr
    ts_key      = tailscale_tailnet_key.router.key
    peer_subnet = each.value.peer_subnet
    router_ip   = each.value.router_ip
    app_ip      = each.value.app_ip
  })

  filename = "${path.module}/generated/${each.key}-docker-compose.yaml"
}

resource "local_file" "server_js" {
  for_each = local.regions

  content = templatefile("${path.module}/orb-templates/server.js.tftpl", {
    region_name  = each.key
    cidr         = each.value.cidr
    app_ip       = each.value.app_ip
    peer_subnet  = each.value.peer_subnet
    peer_app_ip  = each.key == "us" ? local.regions["eu"].app_ip : local.regions["us"].app_ip
    peer_region  = each.key == "us" ? "eu" : "us"
  })

  filename = "${path.module}/generated/${each.key}-server.js"
}
