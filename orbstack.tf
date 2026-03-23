resource "orbstack_machine" "legacy_nodes" {
  for_each = local.regions
  name     = each.value.name
  image    = "ubuntu:jammy"

  cloud_init = templatefile("${path.module}/orb-templates/bootstrap-vm.yaml.tftpl", {})
}