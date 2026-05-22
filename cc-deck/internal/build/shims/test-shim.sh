#!/usr/bin/env bash
#
# Test the LD_PRELOAD getifaddrs shim against a seccomp profile
# that blocks AF_NETLINK (simulating OpenShell's supervisor).
#
# Prerequisites: podman
# Run from the shims/ directory.
#
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
IMAGE="cc-deck-shim-test:latest"
SECCOMP="${SCRIPT_DIR}/test-seccomp.json"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

pass=0
fail=0

log_pass() { echo -e "${GREEN}PASS${NC}: $1"; ((pass++)); }
log_fail() { echo -e "${RED}FAIL${NC}: $1"; ((fail++)); }
log_info() { echo -e "${YELLOW}INFO${NC}: $1"; }

# ── Build the test image ──────────────────────────────────────────
log_info "Building test image..."
podman build -t "${IMAGE}" -f "${SCRIPT_DIR}/Containerfile.test" "${SCRIPT_DIR}" 2>&1 | tail -5

# ── Test 1: Baseline (no seccomp) ── should work ─────────────────
log_info "Test 1: Node.js os.networkInterfaces() WITHOUT seccomp restriction"
if podman run --rm "${IMAGE}" node /opt/test-shim.js 2>&1; then
    log_pass "Baseline works without seccomp"
else
    log_fail "Baseline failed (unexpected)"
fi

# ── Test 2: With seccomp, no shim ── should fail ─────────────────
log_info "Test 2: Node.js os.networkInterfaces() WITH seccomp (AF_NETLINK blocked), NO shim"
if output=$(podman run --rm --security-opt seccomp="${SECCOMP}" "${IMAGE}" \
    node /opt/test-shim.js 2>&1); then
    log_fail "Should have failed with seccomp but didn't. Output: ${output}"
else
    if echo "${output}" | grep -qi "FAIL.*networkInterfaces\|getifaddrs\|Unknown system error"; then
        log_pass "Correctly fails with AF_NETLINK blocked: $(echo "${output}" | head -3)"
    else
        log_fail "Failed but with unexpected error: ${output}"
    fi
fi

# ── Test 3: With seccomp + shim ── should work ───────────────────
log_info "Test 3: Node.js os.networkInterfaces() WITH seccomp + LD_PRELOAD shim"
if output=$(podman run --rm --security-opt seccomp="${SECCOMP}" \
    -e LD_PRELOAD=/usr/local/lib/getifaddrs_shim.so \
    "${IMAGE}" node /opt/test-shim.js 2>&1); then
    if echo "${output}" | grep -q "PASS.*networkInterfaces"; then
        log_pass "Shim bypasses AF_NETLINK block: $(echo "${output}" | head -3)"
    else
        log_pass "Shim works (exit 0): $(echo "${output}" | head -3)"
    fi
else
    log_fail "Shim did not fix the issue. Output: ${output}"
fi

# ── Test 4: Verify shim returns lo interface ──────────────────────
log_info "Test 4: Verify shim returns synthetic loopback interface"
if output=$(podman run --rm --security-opt seccomp="${SECCOMP}" \
    -e LD_PRELOAD=/usr/local/lib/getifaddrs_shim.so \
    "${IMAGE}" node -e "console.log(JSON.stringify(require('os').networkInterfaces()))" 2>&1); then
    if echo "${output}" | grep -q '"lo"'; then
        log_pass "Shim returns lo interface: ${output}"
    else
        log_fail "Shim returned unexpected interfaces: ${output}"
    fi
else
    log_fail "Shim crashed: ${output}"
fi

# ── Test 5: Verify shim is safe under repeated calls ─────────────
log_info "Test 5: Repeated getifaddrs calls (memory safety)"
if output=$(podman run --rm --security-opt seccomp="${SECCOMP}" \
    -e LD_PRELOAD=/usr/local/lib/getifaddrs_shim.so \
    "${IMAGE}" node -e "
        for (let i = 0; i < 1000; i++) require('os').networkInterfaces();
        console.log('PASS: 1000 calls completed');
    " 2>&1); then
    log_pass "Repeated calls safe: $(echo "${output}" | tail -1)"
else
    log_fail "Repeated calls crashed: ${output}"
fi

# ── Summary ───────────────────────────────────────────────────────
echo ""
echo "════════════════════════════════════════"
echo -e "Results: ${GREEN}${pass} passed${NC}, ${RED}${fail} failed${NC}"
echo "════════════════════════════════════════"

# Cleanup
podman rmi "${IMAGE}" >/dev/null 2>&1 || true

exit "${fail}"
