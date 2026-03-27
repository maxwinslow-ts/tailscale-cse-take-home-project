#!/usr/bin/env bash
set -euo pipefail
# ═══════════════════════════════════════════════════════════════════════
# Reads Terraform outputs → writes Ansible inventory.yml
# Run from repo root: bash ansible/generate-inventory.sh
# ═══════════════════════════════════════════════════════════════════════
cd "$(dirname "$0")/.."

EU_DB_IP=$(terraform output -raw eu_db_ip)
EU_ROUTER_IP=$(terraform output -raw eu_router_ip)
US_APP_IP=$(terraform output -raw us_app_ip)
TS_KEY_DB=$(terraform output -raw ts_key_database)
TS_KEY_ROUTER=$(terraform output -raw ts_key_eu_router)
TS_KEY_APP=$(terraform output -raw ts_key_app_server)

cat > ansible/inventory.yml <<EOF
all:
  children:
    eu:
      hosts:
        eu-db:
          ansible_host: root@eu-db@orb
          ansible_user: root@eu-db
          orb_bridge_ip: "${EU_DB_IP}"
          ts_authkey: "${TS_KEY_DB}"
        eu-router:
          ansible_host: root@eu-router@orb
          ansible_user: root@eu-router
          orb_bridge_ip: "${EU_ROUTER_IP}"
          ts_authkey: "${TS_KEY_ROUTER}"
          eu_db_bridge_ip: "${EU_DB_IP}"
    us:
      hosts:
        us-app:
          ansible_host: root@us-app@orb
          ansible_user: root@us-app
          orb_bridge_ip: "${US_APP_IP}"
          ts_authkey: "${TS_KEY_APP}"
          eu_db_bridge_ip: "${EU_DB_IP}"
          eu_router_bridge_ip: "${EU_ROUTER_IP}"
  vars:
    ansible_ssh_private_key_file: ~/.orbstack/ssh/id_ed25519
EOF

echo "✓ Inventory written to ansible/inventory.yml"
