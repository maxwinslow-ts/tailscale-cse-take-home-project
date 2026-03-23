#!/usr/bin/env bash
set -euo pipefail

GREEN='\033[0;32m'  CYAN='\033[0;36m'  BOLD='\033[1m'  RESET='\033[0m'
step() { printf "\n${CYAN}${BOLD}── %s ──${RESET}\n" "$1"; }

# ── 1. Provision VMs & render templates ───────────────
step "1/6  Terraform Apply"
terraform apply -auto-approve

# ── 2. Pull Docker images on Mac host ────────────────
step "2/6  Pull Docker Images"
docker pull tailscale/tailscale:v1.94.2
docker pull node:18-alpine

# ── 3. Wait for Docker daemon inside VMs ─────────────
step "3/6  Waiting for Docker on VMs"
for vm in legacy-us legacy-eu; do
  printf "  Waiting for %s... " "$vm"
  until orb run -m "$vm" sudo docker info >/dev/null 2>&1; do sleep 3; done
  printf "${GREEN}ready${RESET}\n"
done

# ── 4. Load images into each VM ──────────────────────
step "4/6  Loading Images into VMs"
for vm in legacy-us legacy-eu; do
  printf "  Loading into %s... " "$vm"
  docker save tailscale/tailscale:v1.94.2 node:18-alpine | orb run -m "$vm" sudo docker load >/dev/null
  printf "${GREEN}done${RESET}\n"
done

# ── 5. Push compose + server.js to each VM ───────────
step "5/6  Pushing Files to VMs"
for region in us eu; do
  vm="legacy-${region}"
  printf "  Pushing to %s... " "$vm"
  orb push -m "$vm" "generated/${region}-docker-compose.yaml" /tmp/docker-compose.yaml
  orb push -m "$vm" "generated/${region}-server.js" /tmp/server.js
  orb run -m "$vm" sudo mv /tmp/docker-compose.yaml /tmp/server.js /opt/stack/
  printf "${GREEN}done${RESET}\n"
done

# ── 6. Start the stacks ─────────────────────────────
step "6/6  Starting Docker Compose"
for vm in legacy-us legacy-eu; do
  printf "  Starting %s... " "$vm"
  orb run -m "$vm" sudo docker compose -f /opt/stack/docker-compose.yaml up -d >/dev/null 2>&1
  printf "${GREEN}done${RESET}\n"
done

printf "\n${GREEN}${BOLD}Setup complete!${RESET}\n"
printf "  US dashboard: http://legacy-us.orb.local\n"
printf "  EU dashboard: http://legacy-eu.orb.local\n"
printf "  Run ${BOLD}./cmd/verify.sh${RESET} to validate.\n"
