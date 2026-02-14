#!/usr/bin/env bash
set -euo pipefail

# bench_ega_dataset.sh
#
# Benchmark egafetch vs pyEGA3 for a given EGA dataset/file.
# Both tools use the SAME JSON credential/config file via --cf / -cf.
#
# Usage:
#   ./bench_ega_dataset.sh EGAD0000... /path/to/ega_credentials.json /path/to/egafetch /path/to/results_dir
#
# Optional env vars:
#   PYega3_BIN=pyega3
#   RUNS=3
#   SHUFFLE_ORDER=1
#   KEEP_DIRS=0
#
# pyega3 tuning:
#   PYEGA3_MAX_RETRIES=10
#   PYEGA3_RETRY_WAIT=5
#   PYEGA3_EXTRA_OPTS=""           # e.g. "--delete-temp-files"
#
# egafetch tuning:
#   EGA_CHUNK_SIZE=64M
#   EGA_PARALLEL_FILES=8
#   EGA_PARALLEL_CHUNKS=4
#   EGA_RESTART=0

TARGET_ID="${1:-}"      # EGAD... or EGAF...
CRED_JSON="${2:-}"
EGAFETCH_BIN="${3:-}"
RESULTS_DIR="${4:-}"

if [[ -z "$TARGET_ID" || -z "$CRED_JSON" || -z "$EGAFETCH_BIN" || -z "$RESULTS_DIR" ]]; then
  echo "Usage: $0 <EGAD...|EGAF...> <ega_credentials.json> <path/to/egafetch> <results_dir>"
  exit 2
fi

if [[ ! -f "$CRED_JSON" ]]; then
  echo "Credential/config file not found: $CRED_JSON"
  exit 2
fi

if [[ ! -x "$EGAFETCH_BIN" ]]; then
  echo "egafetch binary not executable: $EGAFETCH_BIN"
  exit 2
fi

PYega3_BIN="${PYega3_BIN:-pyega3}"
RUNS="${RUNS:-4}"
SHUFFLE_ORDER="${SHUFFLE_ORDER:-1}"
KEEP_DIRS="${KEEP_DIRS:-0}"

PYEGA3_MAX_RETRIES="${PYEGA3_MAX_RETRIES:-10}"
PYEGA3_RETRY_WAIT="${PYEGA3_RETRY_WAIT:-5}"
PYEGA3_EXTRA_OPTS="${PYEGA3_EXTRA_OPTS:-}"

EGA_CHUNK_SIZE="${EGA_CHUNK_SIZE:-64M}"
EGA_PARALLEL_FILES="${EGA_PARALLEL_FILES:-8}"
EGA_PARALLEL_CHUNKS="${EGA_PARALLEL_CHUNKS:-6}"
EGA_RESTART="${EGA_RESTART:-0}"

mkdir -p "$RESULTS_DIR"/{logs,downloads}

RESULTS_CSV="$RESULTS_DIR/results.csv"
if [[ ! -f "$RESULTS_CSV" ]]; then
  echo "timestamp,run,tool,target_id,elapsed_seconds,exit_code,notes" > "$RESULTS_CSV"
fi

# Choose a portable "time" command
TIME_CMD="time -p"
if command -v gtime >/dev/null 2>&1; then
  TIME_CMD="gtime -p"
elif command -v /usr/bin/time >/dev/null 2>&1; then
  TIME_CMD="/usr/bin/time -p"
fi

ts() { date -u +"%Y-%m-%dT%H:%M:%SZ"; }

clean_dir() {
  local d="$1"
  if [[ "$KEEP_DIRS" == "1" ]]; then
    echo "[INFO] KEEP_DIRS=1 → not removing $d"
    mkdir -p "$d"
    return 0
  fi
  rm -rf "$d"
  mkdir -p "$d"
}

run_cmd_timed() {
  local tool="$1"
  local run_id="$2"
  local log="$3"
  shift 3
  local start end rc elapsed

  start="$(date +%s)"
  set +e
  # shellcheck disable=SC2086
  ( $TIME_CMD "$@" ) >"$log" 2>&1
  rc=$?
  set -e
  end="$(date +%s)"
  elapsed=$(( end - start ))

  echo "$(ts),$run_id,$tool,$TARGET_ID,$elapsed,$rc," >> "$RESULTS_CSV"
  return "$rc"
}

# Record meta info
{
  echo "== environment =="
  echo "date: $(ts)"
  echo "host: $(hostname || true)"
  echo "kernel: $(uname -a || true)"
  echo "pwd: $(pwd)"
  echo ""
  echo "== versions =="
  echo "egafetch: $("$EGAFETCH_BIN" --version 2>/dev/null || true)"
  echo "pyega3: $("$PYega3_BIN" --version 2>/dev/null || true)"
  echo "python: $(python3 --version 2>/dev/null || true)"
  echo ""
  echo "== inputs =="
  echo "target_id: $TARGET_ID"
  echo "cred_json: $CRED_JSON"
  echo ""
  echo "== egafetch settings =="
  echo "chunk_size: $EGA_CHUNK_SIZE"
  echo "parallel_files: $EGA_PARALLEL_FILES"
  echo "parallel_chunks: $EGA_PARALLEL_CHUNKS"
  echo "restart: $EGA_RESTART"
  echo ""
  echo "== pyega3 settings =="
  echo "max_retries: $PYEGA3_MAX_RETRIES"
  echo "retry_wait: $PYEGA3_RETRY_WAIT"
  echo "extra_opts: $PYEGA3_EXTRA_OPTS"
} > "$RESULTS_DIR/logs/meta.txt"

run_egafetch() {
  local run_id="$1"
  local outdir="$RESULTS_DIR/downloads/egafetch_run${run_id}"
  local log="$RESULTS_DIR/logs/egafetch_run${run_id}.log"

  clean_dir "$outdir"

  echo "[INFO] egafetch: download → $outdir"
  local -a args
  args=(
    "$EGAFETCH_BIN" download "$TARGET_ID"
    --cf "$CRED_JSON"
    -o "$outdir"
    --chunk-size "$EGA_CHUNK_SIZE"
    --parallel-files "$EGA_PARALLEL_FILES"
    --parallel-chunks "$EGA_PARALLEL_CHUNKS"
  )
  if [[ "$EGA_RESTART" == "1" ]]; then
    args+=( --restart )
  fi

  # shellcheck disable=SC2145
  echo "[INFO] egafetch command: ${args[*]}"
  run_cmd_timed "EGAfetch" "$run_id" "$log" "${args[@]}" \
    || echo "[WARN] egafetch returned non-zero (see $log)"
}

run_pyega3() {
  local run_id="$1"
  local outdir="$RESULTS_DIR/downloads/pyega3_run${run_id}"
  local log="$RESULTS_DIR/logs/pyega3_run${run_id}.log"

  clean_dir "$outdir"

  echo "[INFO] pyega3: fetch → $outdir"

  # Safe handling with set -u: always define as an array
  local -a extra_opts=()
  if [[ -n "${PYEGA3_EXTRA_OPTS:-}" ]]; then
    # shellcheck disable=SC2206
    extra_opts=( ${PYEGA3_EXTRA_OPTS} )
  fi

  local -a args
  args=(
    "$PYega3_BIN" -cf "$CRED_JSON" --connections 8 fetch
    --max-retries "$PYEGA3_MAX_RETRIES"
    --retry-wait "$PYEGA3_RETRY_WAIT"
    --output-dir "$outdir"
    #"${extra_opts[@]}"
    "$TARGET_ID"
  )

  # shellcheck disable=SC2145
  echo "[INFO] pyega3 command: ${args[*]}"
  run_cmd_timed "pyEGA3" "$run_id" "$log" "${args[@]}" \
    || echo "[WARN] pyEGA3 returned non-zero (see $log)"
}

for run_id in $(seq 1 "$RUNS"); do
  echo
  echo "============================"
  echo "[INFO] Run $run_id / $RUNS"
  echo "============================"

  if [[ "$SHUFFLE_ORDER" == "1" ]]; then
    if (( run_id % 2 == 1 )); then
      run_egafetch "$run_id"
      run_pyega3 "$run_id"
    else
      run_pyega3 "$run_id"
      run_egafetch "$run_id"
    fi
  else
    run_egafetch "$run_id"
    run_pyega3 "$run_id"
  fi
done

echo
echo "[DONE] Benchmark complete."
echo "Results CSV: $RESULTS_CSV"
echo "Logs: $RESULTS_DIR/logs/"
echo
echo "Tips:"
echo "  - Tune egafetch via EGA_PARALLEL_FILES / EGA_PARALLEL_CHUNKS / EGA_CHUNK_SIZE."
echo "  - Tune pyega3 via PYEGA3_MAX_RETRIES / PYEGA3_RETRY_WAIT (and any extra opts)."
echo "  - Use RUNS=3+ and SHUFFLE_ORDER=1 for more stable comparisons."
