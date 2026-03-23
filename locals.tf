locals {
  # 1. Base Configuration: Define the core identity of each "site"
  base_regions = {
    "us" = {
      name = "legacy-us"
      cidr = "172.21.0.0/24"
    }
    "eu" = {
      name = "legacy-eu"
      cidr = "172.22.0.0/24"
    }
  }

  # 2. Derived Configuration: Calculate the specific values needed for Tailscale and OrbStack
  regions = {
    for key, config in local.base_regions : key => {
      name = config.name
      cidr = config.cidr
      
      # Tailscale Tag (e.g., "tag:legacy-us")
      tag  = "tag:${config.name}"

      # Gateway IP for the internal Docker network (.2 address)
      router_ip = cidrhost(config.cidr, 2)

      # App IP on the internal Docker network (.10 address)
      app_ip = cidrhost(config.cidr, 10)

      # Logic: Finds the CIDR of the region that ISN'T this one.
      # This allows the US router to automatically know the EU subnet, and vice versa.
      peer_subnet = one([
        for k, v in local.base_regions : v.cidr 
        if k != key
      ])
    }
  }
}