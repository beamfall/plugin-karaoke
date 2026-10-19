#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# Line-coverage floor for the module (percent). Raise it as tests grow; never
# lower it to land a change.
COVERAGE_FLOOR=95.0
FUZZ_TARGETS=(FuzzParseFilename FuzzParseLRC FuzzParseUltraStar)

run_fast_gate() {
  echo "== gofmt =="
  fmt_out="$(gofmt -l .)"
  if [ -n "$fmt_out" ]; then
    echo "gofmt: files need formatting:"
    echo "$fmt_out"
    exit 1
  fi

  echo "== go vet =="
  go vet ./...

  echo "== go test + coverage floor =="
  run_coverage

  echo "== golangci-lint =="
  golangci-lint run ./...

  echo "ALL GATES GREEN"
}

run_coverage() {
  profile="$(mktemp)"
  trap 'rm -f "$profile"' RETURN
  go test -count=1 -coverprofile="$profile" ./...
  total="$(go tool cover -func="$profile" | awk '/^total:/ { sub(/%/, "", $3); print $3 }')"
  echo "coverage: ${total}% (floor ${COVERAGE_FLOOR}%)"
  awk -v total="$total" -v floor="$COVERAGE_FLOOR" 'BEGIN { exit !(total + 0 >= floor + 0) }' || {
    echo "coverage ${total}% is below the ${COVERAGE_FLOOR}% floor" >&2
    exit 1
  }
}

run_fuzz() {
  fuzz_time="${FUZZ_TIME:-20s}"
  if [[ ! "$fuzz_time" =~ ^([1-9][0-9]*)(s|x)$ ]]; then
    echo "fuzz: FUZZ_TIME must be a positive whole-second or execution budget, got $fuzz_time" >&2
    exit 1
  fi
  expected="$(printf '%s\n' "${FUZZ_TARGETS[@]}" | LC_ALL=C sort)"
  listed="$(go test -list '^Fuzz[A-Za-z0-9_]*$' .)"
  targets="$(printf '%s\n' "$listed" | awk '/^Fuzz[A-Za-z0-9_]*$/ { print }' | LC_ALL=C sort)"
  if [ "$targets" != "$expected" ]; then
    echo "fuzz: expected exactly these targets:" >&2
    printf '%s\n' "$expected" >&2
    echo "fuzz: discovered:" >&2
    printf '%s\n' "${targets:-none}" >&2
    exit 1
  fi
  for target in "${FUZZ_TARGETS[@]}"; do
    GOMAXPROCS=2 go test -run '^$' -fuzz "^${target}$" -fuzztime="$fuzz_time" .
  done
}

case "${1:-gate}" in
  gate) run_fast_gate ;;
  coverage) run_coverage ;;
  race)
    echo "== race =="
    go test -race -count=1 ./...
    ;;
  fuzz)
    echo "== bounded parser fuzz =="
    run_fuzz
    ;;
  *)
    echo "usage: $0 [gate|coverage|race|fuzz]" >&2
    exit 2
    ;;
esac
