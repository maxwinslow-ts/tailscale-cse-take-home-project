#!/usr/bin/env bash
set -euo pipefail

# ── Colors ──────────────────────────────────────────────
GREEN='\033[0;32m'  RED='\033[0;31m'  YELLOW='\033[1;33m'
CYAN='\033[0;36m'   BOLD='\033[1m'    RESET='\033[0m'

PASS=0  FAIL=0

pass() { ((PASS++)); printf "${GREEN}  ✔ PASS${RESET}  %s\n" "$1"; }
fail() { ((FAIL++)); printf "${RED}  ✘ FAIL${RESET}  %s\n" "$1"; }
header() { printf "\n${CYAN}${BOLD}── %s ──${RESET}\n" "$1"; }

# ── 1. VMs reachable ───────────────────────────────────
header "1. VM Reachability"
for vm in legacy-us legacy-eu; do
  if orb run -m "$vm" true 2>/dev/null; then
    pass "$vm is reachable"
  else
    fail "$vm is NOT reachable"
  fi
done

# ── 2. Docker healthy ─────────────────────────────────
header "2. Docker Daemon"
for vm in legacy-us legacy-eu; do
  if orb run -m "$vm" sudo docker info >/dev/null 2>&1; then
    pass "$vm Docker is running"
  else
    fail "$vm Docker is NOT running"
  fi
done

# ── 3. Stack files present ────────────────────────────
header "3. Stack Files"
for vm in legacy-us legacy-eu; do
  for f in docker-compose.yaml server.js; do
    if orb run -m "$vm" test -f "/opt/stack/$f" 2>/dev/null; then
      pass "$vm /opt/stack/$f exists"
    else
      fail "$vm /opt/stack/$f MISSING"
    fi
  done
done

# ── 4. Containers running ────────────────────────────
header "4. Container Status"
for pair in "legacy-us:us-tailscale-router" "legacy-us:us-app" "legacy-eu:eu-tailscale-router" "legacy-eu:eu-app"; do
  IFS=: read -r vm ctr <<< "$pair"
  status=$(orb run -m "$vm" sudo docker inspect -f '{{.State.Status}}' "$ctr" 2>/dev/null || echo "missing")
  if [[ "$status" == "running" ]]; then
    pass "$ctr is running"
  else
    fail "$ctr status: $status"
  fi
done

# ── 5. Container IPs correct ─────────────────────────
header "5. Container IPs"
check_ip() {
  local vm=$1 ctr=$2 expected=$3
  actual=$(orb run -m "$vm" sudo docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$ctr" 2>/dev/null || echo "")
  if [[ "$actual" == "$expected" ]]; then
    pass "$ctr IP = $actual"
  else
    fail "$ctr IP expected $expected, got '$actual'"
  fi
}
check_ip legacy-us us-tailscale-router 172.21.0.2
check_ip legacy-us us-app              172.21.0.10
check_ip legacy-eu eu-tailscale-router 172.22.0.2
check_ip legacy-eu eu-app              172.22.0.10

# ── 6. Tailscale status ──────────────────────────────
header "6. Tailscale Status"
for pair in "legacy-us:us-tailscale-router:eu-router" "legacy-eu:eu-tailscale-router:us-router"; do
  IFS=: read -r vm ctr peer <<< "$pair"
  ts_out=$(orb run -m "$vm" sudo docker exec "$ctr" tailscale status 2>/dev/null || echo "")
  if echo "$ts_out" | grep -q "$peer"; then
    pass "$ctr sees peer $peer"
  else
    fail "$ctr does NOT see peer $peer"
  fi
done

# ── 7. App route tables ──────────────────────────────
header "7. App Route Tables"
check_route() {
  local vm=$1 ctr=$2 peer_subnet=$3 via=$4
  routes=$(orb run -m "$vm" sudo docker exec "$ctr" ip route 2>/dev/null || echo "")
  if echo "$routes" | grep -q "$peer_subnet via $via"; then
    pass "$ctr routes $peer_subnet via $via"
  else
    fail "$ctr missing route $peer_subnet via $via"
  fi
}
check_route legacy-us us-app 172.22.0.0/24 172.21.0.2
check_route legacy-eu eu-app 172.21.0.0/24 172.22.0.2

# ── 8. Cross-site /health ────────────────────────────
header "8. Cross-Site Connectivity (via Tailscale)"

# US app → EU app
us_to_eu=$(orb run -m legacy-us sudo docker exec us-app curl -sf http://172.22.0.10:3000/health 2>/dev/null || echo "")
if echo "$us_to_eu" | grep -q '"region":"eu"'; then
  pass "US app → EU app /health (via Tailscale)"
else
  fail "US app → EU app /health FAILED"
fi

# EU app → US app
eu_to_us=$(orb run -m legacy-eu sudo docker exec eu-app curl -sf http://172.21.0.10:3000/health 2>/dev/null || echo "")
if echo "$eu_to_us" | grep -q '"region":"us"'; then
  pass "EU app → US app /health (via Tailscale)"
else
  fail "EU app → US app /health FAILED"
fi

# ── 9. SNAT disabled (source IP preserved) ───────────
header "9. Source IP Preservation (SNAT Disabled)"

us_from=$(echo "$us_to_eu" | grep -o '"requestFrom":"[^"]*"' | head -1)
if echo "$us_from" | grep -q "172.21.0.10"; then
  pass "EU sees US app's real IP (172.21.0.10) — SNAT disabled"
else
  fail "EU sees wrong source IP: $us_from (expected 172.21.0.10)"
fi

eu_from=$(echo "$eu_to_us" | grep -o '"requestFrom":"[^"]*"' | head -1)
if echo "$eu_from" | grep -q "172.22.0.10"; then
  pass "US sees EU app's real IP (172.22.0.10) — SNAT disabled"
else
  fail "US sees wrong source IP: $eu_from (expected 172.22.0.10)"
fi

# ── 10. VM host port 80 ──────────────────────────────
header "10. VM Host Port Forwarding (:80)"
for pair in "legacy-us:us" "legacy-eu:eu"; do
  IFS=: read -r vm region <<< "$pair"
  health=$(orb run -m "$vm" curl -sf http://localhost:80/health 2>/dev/null || echo "")
  if echo "$health" | grep -q "\"region\":\"$region\""; then
    pass "$vm:80 → $region app /health"
  else
    fail "$vm:80 → $region app NOT responding"
  fi
done

# ── Summary ───────────────────────────────────────────
printf "\n${BOLD}════════════════════════════════════════${RESET}\n"
printf "${GREEN}  Passed: %d${RESET}  ${RED}  Failed: %d${RESET}\n" "$PASS" "$FAIL"
if [[ $FAIL -eq 0 ]]; then
  printf "${GREEN}${BOLD}  All checks passed! ✅${RESET}\n"
else
  printf "${RED}${BOLD}  Some checks failed. Review output above.${RESET}\n"
fi
printf "${BOLD}════════════════════════════════════════${RESET}\n"

# ── SSH Break-Glass (manual) ─────────────────────────
printf "\n${CYAN}${BOLD}── 11. Tailscale SSH (Break-Glass Access) ──${RESET}\n"
printf "${YELLOW}  Run these commands manually to verify IdP-backed SSH:${RESET}\n\n"
printf "  ${BOLD}tailscale ssh root@us-router -- hostname && echo \"✅ US SSH OK\" || echo \"❌ US SSH FAILED\"${RESET}\n"
printf "  ${BOLD}tailscale ssh root@eu-router -- hostname && echo \"✅ EU SSH OK\" || echo \"❌ EU SSH FAILED\"${RESET}\n\n"