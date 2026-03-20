#!/bin/bash
# Smoke test for cc-deck env commands.
# Runs the compiled binary through a full lifecycle without Zellij.
#
# Usage:
#   make build && ./scripts/smoke-test-env.sh
#   # or with a specific binary:
#   CC_DECK_BIN=/path/to/cc-deck ./scripts/smoke-test-env.sh

set -uo pipefail

# --- Setup ---

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CC_DECK_BIN="${CC_DECK_BIN:-$PROJECT_ROOT/cc-deck/cc-deck}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

STATE_FILE="$TMPDIR/state.yaml"

# Create a zellij stub so LookPath succeeds.
STUB_BIN="$TMPDIR/bin"
mkdir -p "$STUB_BIN"
printf '#!/bin/sh\nexit 0\n' > "$STUB_BIN/zellij"
chmod +x "$STUB_BIN/zellij"

export CC_DECK_STATE_FILE="$STATE_FILE"
export PATH="$STUB_BIN:$PATH"

PASS=0
FAIL=0

# --- Helpers ---

pass() { PASS=$((PASS + 1)); printf "  \033[32mPASS\033[0m %s\n" "$1"; }
fail() { FAIL=$((FAIL + 1)); printf "  \033[31mFAIL\033[0m %s: %s\n" "$1" "$2"; }

run() { "$CC_DECK_BIN" "$@" 2>&1; }

assert_contains() {
    local output="$1" expected="$2" label="$3"
    if echo "$output" | grep -q "$expected"; then
        pass "$label"
    else
        fail "$label" "expected '$expected' in output"
    fi
}

assert_not_contains() {
    local output="$1" unexpected="$2" label="$3"
    if echo "$output" | grep -q "$unexpected"; then
        fail "$label" "unexpected '$unexpected' in output"
    else
        pass "$label"
    fi
}

assert_exit_code() {
    local expected="$1" label="$2"
    shift 2
    local code=0
    "$CC_DECK_BIN" "$@" >/dev/null 2>&1 || code=$?
    if [ "$code" -eq "$expected" ]; then
        pass "$label"
    elif [ "$expected" -ne 0 ] && [ "$code" -ne 0 ]; then
        pass "$label"
    else
        fail "$label" "expected exit $expected, got $code"
    fi
}

# --- Verify binary ---

printf "\n\033[1m=== cc-deck env smoke tests ===\033[0m\n\n"

if [ ! -x "$CC_DECK_BIN" ]; then
    echo "Binary not found at $CC_DECK_BIN"
    echo "Run 'make build' first or set CC_DECK_BIN"
    exit 1
fi

printf "\033[1m1. Version & help\033[0m\n"
out=$(run version)
assert_contains "$out" "cc-deck" "version output"
out=$(run env --help)
assert_contains "$out" "create" "help mentions create"

# --- Create ---

printf "\n\033[1m2. Create environments\033[0m\n"
out=$(run env create smoke-test --type local)
assert_contains "$out" "created" "create local env"

assert_exit_code 1 "create duplicate rejects" env create smoke-test --type local
assert_exit_code 1 "create invalid name rejects" env create INVALID --type local
assert_exit_code 1 "create unsupported type" env create podtest --type podman

# --- List ---

printf "\n\033[1m3. List environments\033[0m\n"
out=$(run env list)
assert_contains "$out" "smoke-test" "list shows env"
assert_contains "$out" "local" "list shows type"

out=$(run env list -o json)
assert_contains "$out" '"Name": "smoke-test"' "list JSON output"

out=$(run env list --type local)
assert_contains "$out" "smoke-test" "list filter local"

out=$(run env list --type podman)
assert_not_contains "$out" "smoke-test" "list filter podman excludes local"

# --- Status ---

printf "\n\033[1m4. Status\033[0m\n"
out=$(run env status smoke-test)
assert_contains "$out" "smoke-test" "status shows name"
assert_contains "$out" "local" "status shows type"

out=$(run env status smoke-test -o json)
assert_contains "$out" '"name": "smoke-test"' "status JSON"

assert_exit_code 1 "status not found" env status nonexistent

# --- Stop/Start ---

printf "\n\033[1m5. Stop/Start (local = not supported)\033[0m\n"
# Note: after list, reconciliation may change state to unknown.
# The stop command checks state==running first, so we test the actual
# error message (either "not running" or "not supported").
out=$(run env stop smoke-test)
if echo "$out" | grep -q "not supported\|not running"; then
    pass "stop rejects local env"
else
    fail "stop rejects local env" "unexpected output: $out"
fi

assert_exit_code 1 "start non-stopped env rejects" env start smoke-test

# --- Multiple envs ---

printf "\n\033[1m6. Multiple environments\033[0m\n"
run env create alpha >/dev/null
run env create beta >/dev/null
out=$(run env list)
assert_contains "$out" "alpha" "multi: alpha listed"
assert_contains "$out" "beta" "multi: beta listed"
assert_contains "$out" "smoke-test" "multi: smoke-test still listed"

# --- Delete ---

printf "\n\033[1m7. Delete\033[0m\n"
out=$(run env delete beta --force)
assert_contains "$out" "deleted" "delete beta"

out=$(run env list)
assert_not_contains "$out" "beta" "beta gone from list"
assert_contains "$out" "alpha" "alpha still listed"

assert_exit_code 1 "delete not found" env delete ghost --force

# --- State file ---

printf "\n\033[1m8. State persistence\033[0m\n"
if [ -f "$STATE_FILE" ]; then
    pass "state file exists"
    content=$(cat "$STATE_FILE")
    assert_contains "$content" "smoke-test" "state file has env"
else
    fail "state file exists" "file not found at $STATE_FILE"
fi

# --- Cleanup cycle ---

printf "\n\033[1m9. Full cleanup\033[0m\n"
run env delete smoke-test --force >/dev/null
run env delete alpha --force >/dev/null
out=$(run env list)
assert_contains "$out" "No environments found" "all deleted"

# --- Summary ---

TOTAL=$((PASS + FAIL))
printf "\n\033[1m=== Results: %d/%d passed" "$PASS" "$TOTAL"
if [ "$FAIL" -gt 0 ]; then
    printf " (\033[31m%d failed\033[0m)" "$FAIL"
fi
printf " ===\033[0m\n\n"

exit "$FAIL"
