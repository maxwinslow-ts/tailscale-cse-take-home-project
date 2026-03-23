#!/usr/bin/env bash
set -euo pipefail

GREEN='\033[0;32m'  RED='\033[0;31m'  CYAN='\033[0;36m'  BOLD='\033[1m'  RESET='\033[0m'
step() { printf "\n${CYAN}${BOLD}── %s ──${RESET}\n" "$1"; }

# ── Read Tailscale credentials from terraform.tfvars ──
TFVARS="$(cd "$(dirname "$0")/.." && pwd)/terraform.tfvars"
TS_API_KEY=$(grep 'tailscale_api_key' "$TFVARS" | sed 's/.*= *"\(.*\)"/\1/')
TS_TAILNET=$(grep 'tailscale_tailnet' "$TFVARS" | sed 's/.*= *"\(.*\)"/\1/')

# ── 1. Stop Docker Compose stacks ────────────────────
step "1/3  Stopping Docker Compose"
for vm in legacy-us legacy-eu; do
  printf "  Stopping %s... " "$vm"
  orb run -m "$vm" sudo docker compose -f /opt/stack/docker-compose.yaml down >/dev/null 2>&1 || true
  printf "${GREEN}done${RESET}\n"
done

# ── 2. Remove Tailscale router nodes (tag:router only) ─
step "2/3  Removing Tailscale Router Nodes"

# Get all devices, one per line (each JSON object flattened)
device_ids=$(curl -sf \
  -H "Authorization: Bearer ${TS_API_KEY}" \
  "https://api.tailscale.com/api/v2/tailnet/${TS_TAILNET}/devices?fields=all" 2>/dev/null \
  | grep -o '"nodeId":"[^"]*"[^}]*' \
  | grep '"tag:router"' \
  | grep -o '"nodeId":"[^"]*"' \
  | sed 's/"nodeId":"//;s/"//g') || true

if [[ -z "$device_ids" ]]; then
  printf "  ⊘ No devices with tag:router found\n"
else
  for device_id in $device_ids; do
    if curl -sf -X DELETE \
      -H "Authorization: Bearer ${TS_API_KEY}" \
      "https://api.tailscale.com/api/v2/device/${device_id}" >/dev/null 2>&1; then
      printf "${GREEN}  ✔${RESET} Deleted device %s\n" "$device_id"
    else
      printf "${RED}  ✘${RESET} Failed to delete device %s\n" "$device_id"
    fi
  done
fi

# ── 3. Destroy Terraform resources ───────────────────
step "3/3  Terraform Destroy"
terraform destroy -auto-approve

printf "\n${GREEN}${BOLD}Teardown complete.${RESET}\n"
