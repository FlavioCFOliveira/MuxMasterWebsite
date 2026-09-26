#!/usr/bin/env bash
# Benchmark campaign 2026-09-26 — exact commands.
#
# Usage:
#   ./run.sh setup     create the isolated tag checkouts and build every test binary
#   ./run.sh measure   run every suite, interleaved, 10 rounds of -test.count=1
#   ./run.sh stats     run benchstat over the raw outputs
#   ./run.sh cleanup   remove the isolated tag checkouts
#
# SCRATCH is any empty directory outside the upstream repository.
# UPSTREAM is a clone of https://github.com/FlavioCFOliveira/MuxMaster.
set -euo pipefail

UPSTREAM=${UPSTREAM:-/data/dev/github.com/FlavioCFOliveira/MuxMaster}
SCRATCH=${SCRATCH:-/tmp/claude-1000/-data-dev-github-com-FlavioCFOliveira-MuxMasterWebsite/be49749c-37dc-4fa8-9963-5ba37ff7cbc9/scratchpad}
ARCHIVE=$(cd "$(dirname "$0")" && pwd)
BIN=$SCRATCH/bin
RAW=$ARCHIVE/raw
ROUNDS=${ROUNDS:-10}

# One pinned toolchain for every build: never switch toolchains, never download modules.
export GOTOOLCHAIN=local GOPROXY=off GOFLAGS=

setup() {
  git -C "$UPSTREAM" worktree add --detach "$SCRATCH/v110" v1.1.0
  git -C "$UPSTREAM" worktree add --detach "$SCRATCH/v130" v1.3.0
  mkdir -p "$BIN"
  (cd "$SCRATCH/v110" && go test -c -o "$BIN/root-v110.test" . \
                      && go test -c -o "$BIN/pa-v110.test" ./reports/perf-audit-2026-05-12/)
  (cd "$SCRATCH/v130" && go test -c -o "$BIN/root-v130.test" . \
                      && go test -c -o "$BIN/pa-v130.test" ./reports/perf-audit-2026-05-12/ \
                      && go test -c -o "$BIN/mw-v130.test" ./middleware/)
  # The competitor suite's go.mod, go.sum and test files are git-ignored upstream and
  # are not part of the v1.3.0 tag (the tag tracks only competitor/vendor/). The copies
  # used are archived in competitor-suite/. -mod=mod makes the build use the
  # `replace ... => ../` directive (the v1.3.0 checkout) instead of the stale vendored
  # copy of MuxMaster under competitor/vendor/.
  cp "$ARCHIVE"/competitor-suite/{go.mod,go.sum,bench_test.go,route_existence_timing_test.go} \
     "$SCRATCH/v130/competitor/"
  (cd "$SCRATCH/v130/competitor" && go test -mod=mod -c -o "$BIN/comp-v130.test" .)
  (cd "$SCRATCH/v130/reports/perf-lab-2026-09-24/waste-hunt/bench" \
     && go test -mod=mod -c -o "$BIN/wh-v130.test" .)
}

# run <binary> <package dir> <output file>
run() {
  local bin=$1 dir=$2 out=$3
  {
    printf '%s %s load=%s\n' "$(date -Is)" "$(basename "$out")" "$(cut -d' ' -f1-3 /proc/loadavg)"
    ps -eo pcpu,comm --sort=-pcpu | sed -n '2,6p' | tr -s ' ' | paste -sd';'
  } >> "$RAW/host-load.log"
  local rc=0
  (cd "$dir" && "$bin" -test.run='^$' -test.bench=. -test.benchmem \
       -test.count=1 -test.benchtime=1s) > "$out" 2>&1 || rc=$?
  printf '%s %s exit=%s last=%s\n' "$(date -Is)" "$(basename "$out")" "$rc" \
    "$(tail -n1 "$out")" >> "$RAW/run-status.log"
}

measure() {
  mkdir -p "$RAW/rounds"
  local r
  # Phase 1: v1.1.0 versus v1.3.0. Per round: root package and perf-audit harness for
  # both versions, plus a second v1.3.0 root run (same binary) for the A/A noise floor.
  # Odd rounds start with v1.1.0, even rounds with v1.3.0 (ABBA order) to cancel drift.
  for r in $(seq -w 1 "$ROUNDS"); do
    if (( 10#$r % 2 )); then
      run "$BIN/root-v110.test" "$SCRATCH/v110" "$RAW/rounds/root-v110-r$r.txt"
      run "$BIN/root-v130.test" "$SCRATCH/v130" "$RAW/rounds/root-v130-r$r.txt"
      run "$BIN/pa-v110.test"   "$SCRATCH/v110/reports/perf-audit-2026-05-12" "$RAW/rounds/pa-v110-r$r.txt"
      run "$BIN/pa-v130.test"   "$SCRATCH/v130/reports/perf-audit-2026-05-12" "$RAW/rounds/pa-v130-r$r.txt"
      run "$BIN/root-v130.test" "$SCRATCH/v130" "$RAW/rounds/root-v130aa-r$r.txt"
    else
      run "$BIN/root-v130.test" "$SCRATCH/v130" "$RAW/rounds/root-v130aa-r$r.txt"
      run "$BIN/pa-v130.test"   "$SCRATCH/v130/reports/perf-audit-2026-05-12" "$RAW/rounds/pa-v130-r$r.txt"
      run "$BIN/pa-v110.test"   "$SCRATCH/v110/reports/perf-audit-2026-05-12" "$RAW/rounds/pa-v110-r$r.txt"
      run "$BIN/root-v130.test" "$SCRATCH/v130" "$RAW/rounds/root-v130-r$r.txt"
      run "$BIN/root-v110.test" "$SCRATCH/v110" "$RAW/rounds/root-v110-r$r.txt"
    fi
  done
  # Phase 2: v1.3.0 only — competitor suite, middleware package, waste-hunt harness.
  for r in $(seq -w 1 "$ROUNDS"); do
    run "$BIN/comp-v130.test" "$SCRATCH/v130/competitor" "$RAW/rounds/competitor-v130-r$r.txt"
    run "$BIN/mw-v130.test"   "$SCRATCH/v130/middleware" "$RAW/rounds/middleware-v130-r$r.txt"
    run "$BIN/wh-v130.test"   "$SCRATCH/v130/reports/perf-lab-2026-09-24/waste-hunt/bench" "$RAW/rounds/wastehunt-v130-r$r.txt"
  done
  local s
  for s in root-v110 root-v130 root-v130aa pa-v110 pa-v130 competitor-v130 middleware-v130 wastehunt-v130; do
    cat "$RAW"/rounds/"$s"-r*.txt > "$RAW/$s.txt"
  done
}

stats() {
  local B=$ARCHIVE/benchstat
  mkdir -p "$B/csv"
  cd "$RAW"   # relative file names become the benchstat column labels
  benchstat "root-v110.txt" "root-v130.txt"    > "$B/root-v110-vs-v130.txt"
  benchstat "root-v130.txt" "root-v130aa.txt"  > "$B/root-v130-aa-noise-floor.txt"
  # The perf-audit harness logs a JWTAuth warning to stderr, which the raw output
  # captures; strip those lines into *-clean.txt before benchstat. Values are untouched.
  local v
  for v in v110 v130; do
    "$ARCHIVE/strip-log-lines.sh" < "pa-$v.txt" > "pa-$v-clean.txt"
  done
  benchstat "pa-v110-clean.txt" "pa-v130-clean.txt" > "$B/perfaudit-v110-vs-v130.txt"
  benchstat "middleware-v130.txt"                   > "$B/middleware-v130.txt"
  benchstat "wastehunt-v130.txt"                    > "$B/wastehunt-v130.txt"
  benchstat "competitor-v130.txt"                   > "$B/competitor-v130.txt"
  # Rename competitor benchmarks to Benchmark<Category>/router=<Router> so that benchstat
  # can compare routers per category with a significance test. Values are untouched.
  "$ARCHIVE/competitor-rename.sh" < "competitor-v130.txt" > "competitor-v130-by-router.txt"
  local base
  others() { local r; for r in MuxMaster MuxMasterPooled MuxMasterFast HTTProuter BunRouter Chi GorillaMux; do [ "$r" = "$1" ] || printf '%s ' "$r"; done; }
  for base in MuxMaster MuxMasterPooled MuxMasterFast HTTProuter; do
    benchstat -col "/router@($base $(others "$base"))" \
      "competitor-v130-by-router.txt" > "$B/competitor-v130-vs-$base.txt"
    benchstat -format csv -col "/router@($base $(others "$base"))" \
      "competitor-v130-by-router.txt" > "$B/csv/competitor-v130-vs-$base.csv"
  done
  # The same comparisons in CSV form; the README tables are generated from these files.
  benchstat -format csv "root-v110.txt" "root-v130.txt"            > "$B/csv/root-v110-vs-v130.csv"
  benchstat -format csv "root-v130.txt" "root-v130aa.txt"          > "$B/csv/root-v130-aa-noise-floor.csv"
  benchstat -format csv "pa-v110-clean.txt" "pa-v130-clean.txt"    > "$B/csv/perfaudit-v110-vs-v130.csv"
  benchstat -format csv "middleware-v130.txt"                      > "$B/csv/middleware-v130.csv"
  benchstat -format csv "wastehunt-v130.txt"                       > "$B/csv/wastehunt-v130.csv"
}

cleanup() {
  git -C "$UPSTREAM" worktree remove "$SCRATCH/v110"
  git -C "$UPSTREAM" worktree remove "$SCRATCH/v130"
}

"${1:?usage: run.sh setup|measure|stats|cleanup}"
