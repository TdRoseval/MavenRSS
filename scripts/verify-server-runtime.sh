#!/usr/bin/env bash

set -euo pipefail

BINARY_PATH="${1:-}"
PORT="${2:-18123}"
WORK_DIR="${3:-}"
HOST="${HOST:-127.0.0.1}"
STARTUP_TIMEOUT_SECONDS="${STARTUP_TIMEOUT_SECONDS:-45}"

if [[ -z "${BINARY_PATH}" ]]; then
  echo "Usage: $0 <binary-path> [port] [work-dir]"
  exit 1
fi

if [[ ! -f "${BINARY_PATH}" ]]; then
  echo "Binary not found: ${BINARY_PATH}"
  exit 1
fi

if [[ -z "${WORK_DIR}" ]]; then
  WORK_DIR="$(mktemp -d)"
else
  mkdir -p "${WORK_DIR}"
fi

STDOUT_LOG="${WORK_DIR}/server.stdout.log"
STDERR_LOG="${WORK_DIR}/server.stderr.log"
APP_LOG="${WORK_DIR}/data/logs/debug.log"
RESPONSE_FILE="${WORK_DIR}/version.json"
SERVER_PID=""

cleanup() {
  if [[ -n "${SERVER_PID}" ]] && kill -0 "${SERVER_PID}" 2>/dev/null; then
    kill "${SERVER_PID}" 2>/dev/null || true
    wait "${SERVER_PID}" 2>/dev/null || true
  fi
}

print_log_file() {
  local file_path="$1"
  local label="$2"
  if [[ -f "${file_path}" ]]; then
    echo "===== ${label} ====="
    tail -n 200 "${file_path}" || true
  fi
}

print_diagnostics() {
  echo "Runtime verification failed"
  echo "Binary: ${BINARY_PATH}"
  echo "Work directory: ${WORK_DIR}"
  echo "Port: ${PORT}"
  uname -a || true
  file "${BINARY_PATH}" || true
  ls -lah "$(dirname "${BINARY_PATH}")" || true
  ls -lah "${WORK_DIR}" || true
  print_log_file "${STDOUT_LOG}" "server.stdout.log"
  print_log_file "${STDERR_LOG}" "server.stderr.log"
  print_log_file "${APP_LOG}" "debug.log"
}

trap cleanup EXIT

ABSOLUTE_BINARY_PATH="$(cd "$(dirname "${BINARY_PATH}")" && pwd)/$(basename "${BINARY_PATH}")"

pushd "${WORK_DIR}" >/dev/null
"${ABSOLUTE_BINARY_PATH}" -host "${HOST}" -port "${PORT}" >"${STDOUT_LOG}" 2>"${STDERR_LOG}" &
SERVER_PID=$!
popd >/dev/null

echo "Started server process ${SERVER_PID} using ${ABSOLUTE_BINARY_PATH}"

deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  if ! kill -0 "${SERVER_PID}" 2>/dev/null; then
    print_diagnostics
    exit 1
  fi

  if curl --fail --silent --show-error "http://${HOST}:${PORT}/api/version" >"${RESPONSE_FILE}" 2>/dev/null; then
    break
  fi

  sleep 1
done

if [[ ! -s "${RESPONSE_FILE}" ]]; then
  print_diagnostics
  exit 1
fi

if [[ ! -f "${WORK_DIR}/data/rss.db" ]]; then
  echo "Expected database file was not created"
  print_diagnostics
  exit 1
fi

if [[ ! -f "${APP_LOG}" ]]; then
  echo "Expected debug log was not created"
  print_diagnostics
  exit 1
fi

if ! grep -Fq "SQLite self-check passed" "${APP_LOG}"; then
  echo "SQLite self-check success log not found"
  print_diagnostics
  exit 1
fi

echo "Runtime verification response:"
cat "${RESPONSE_FILE}"
echo
echo "Runtime verification passed"
