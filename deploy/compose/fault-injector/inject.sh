#!/usr/bin/env bash
set -euo pipefail

TARGET="${CHECKOUT_URL:-http://checkout-sim:18080}"
TOKEN="${CHAOS_TOKEN:-dev-chaos-token}"
HDR=(-H "X-Chaos-Token: ${TOKEN}" -H "Content-Type: application/json")

log() { echo "[fault-injector] $*"; }

chaos_post() {
  local body=$1
  curl -sS -f -X POST "${TARGET}/internal/chaos" "${HDR[@]}" -d "${body}"
}

reset() {
  log "reset chaos"
  curl -sS -f -X POST "${TARGET}/internal/chaos/reset" "${HDR[@]}" -o /dev/null
}

traffic() {
  local n=${1:-40}
  log "generate ${n} checkout requests"
  for _ in $(seq 1 "${n}"); do
    curl -sS -o /dev/null "${TARGET}/api/checkout" || true
  done
}

scenario_latency() {
  log "inject latency_ms=900 (3 min)"
  chaos_post '{"latency_ms":900,"error_pct":0}'
  sleep 180
  reset
}

scenario_errors() {
  log "inject error_pct=40 (2 min)"
  chaos_post '{"latency_ms":0,"error_pct":40}'
  sleep 120
  reset
}

scenario_combo() {
  log "inject latency+errors (90s)"
  chaos_post '{"latency_ms":400,"error_pct":25}'
  sleep 90
  reset
}

case "${1:-help}" in
  latency) scenario_latency ;;
  errors)  scenario_errors ;;
  combo)   scenario_combo ;;
  reset)   reset ;;
  traffic) traffic "${2:-50}" ;;
  demo)
    log "full demo: traffic + latency + traffic + errors + combo"
    traffic 30
    scenario_latency
    traffic 40
    scenario_errors
    traffic 40
    scenario_combo
    traffic 30
    log "done"
    ;;
  help|*)
    echo "Usage: $0 {demo|latency|errors|combo|reset|traffic [n]}"
    exit 1
    ;;
esac
