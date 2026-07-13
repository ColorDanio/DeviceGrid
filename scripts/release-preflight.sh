#!/usr/bin/env bash
set -euo pipefail

binary=${1:-bin/devicegrid-server}
port=${DG_PREFLIGHT_PORT:-31800}
workdir=$(mktemp -d)
pid=''

cleanup() {
  if [[ -n "$pid" ]]; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -rf "$workdir"
}
trap cleanup EXIT

if [[ ! -x "$binary" ]]; then
  echo "release preflight: executable not found: $binary" >&2
  exit 1
fi

cp configs/config.yaml "$workdir/config.yaml"
sed -i \
  -e "s/port: 3000/port: $port/" \
  -e "s#path: \"./data/device_grid.db\"#path: \"$workdir/device_grid.db\"#" \
  -e 's/grpc_port: 9090/grpc_port: 31900/' \
  "$workdir/config.yaml"

DG_CONFIG_PATH="$workdir/config.yaml" "$binary" >"$workdir/server.log" 2>&1 &
pid=$!

for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:$port/healthz" >/dev/null; then
    break
  fi
  sleep 1
done

curl -fsS "http://127.0.0.1:$port/healthz" >/dev/null
curl -fsS "http://127.0.0.1:$port/nonexistent-spa-route" | grep -q '<div id="app">'

echo "release preflight: health endpoint and embedded SPA passed"
