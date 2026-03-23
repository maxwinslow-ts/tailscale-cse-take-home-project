#!/usr/bin/env bash
set -euo pipefail

GREEN='\033[0;32m'  CYAN='\033[0;36m'  BOLD='\033[1m'  RESET='\033[0m'
step() { printf "\n${CYAN}${BOLD}── %s ──${RESET}\n" "$1"; }

# ── 1. Re-render templates ───────────────────────────
step "1/3  Terraform Apply (regenerate files)"
terraform apply -auto-approve

# ── 2. Push updated files to each VM ─────────────────
step "2/3  Pushing Files to VMs"
for region in us eu; do
  vm="legacy-${region}"
  printf "  Pushing to %s... " "$vm"
  orb push -m "$vm" "generated/${region}-docker-compose.yaml" /tmp/docker-compose.yaml
  orb push -m "$vm" "generated/${region}-server.js" /tmp/server.js
  orb run -m "$vm" sudo mv /tmp/docker-compose.yaml /tmp/server.js /opt/stack/
  printf "${GREEN}done${RESET}\n"
done

# ── 3. Restart the stacks ───────────────────────────
step "3/3  Restarting Docker Compose"
for vm in legacy-us legacy-eu; do
  printf "  Restarting %s... " "$vm"
  orb run -m "$vm" sudo docker compose -f /opt/stack/docker-compose.yaml up -d --force-recreate >/dev/null 2>&1
  printf "${GREEN}done${RESET}\n"
done

printf "\n${GREEN}${BOLD}Redeploy complete!${RESET}\n"
printf "  US dashboard: http://legacy-us.orb.local\n"
printf "  EU dashboard: http://legacy-eu.orb.local\n"
printf "  Run ${BOLD}./cmd/verify.sh${RESET} to validate.\n"