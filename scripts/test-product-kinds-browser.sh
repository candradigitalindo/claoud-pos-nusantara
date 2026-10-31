#!/usr/bin/env bash
# Build and browser regressions with synthetic API responses, no host ports.
set -euo pipefail
repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
artifact_dir=""
modules_dir=""
offline=false
while (($#)); do
  case "$1" in
    --artifacts|--playwright-modules)
      (($# >= 2)) || exit 2
      case "$1" in --artifacts) artifact_dir=$2;; --playwright-modules) modules_dir=$2;; esac
      shift 2;;
    --offline) offline=true; shift;;
    *) echo 'Usage: test-product-kinds-browser.sh --artifacts NEW_DIR --playwright-modules DIR [--offline]' >&2; exit 2;;
  esac
done
[[ -n $artifact_dir && -f $modules_dir/playwright/package.json && -f $modules_dir/playwright-core/package.json ]] || {
  echo 'Provide a new artifact directory and node_modules containing playwright and playwright-core 1.47.2.' >&2; exit 2;
}
python3 - "$modules_dir/playwright/package.json" "$modules_dir/playwright-core/package.json" <<'PY'
import json, sys
for path in sys.argv[1:]:
    assert json.load(open(path))['version'] == '1.47.2', 'Playwright and core must match browser image 1.47.2'
PY
mkdir -p "$artifact_dir"
artifact_dir=$(cd "$artifact_dir" && pwd)
modules_dir=$(cd "$modules_dir" && pwd)
[[ ! -e $artifact_dir/run-id ]] || { echo 'Artifact directory already used' >&2; exit 2; }
run_id="pos-kind-ui-$(date -u +%Y%m%d%H%M%S)-$$"
printf '%s\n' "$run_id" > "$artifact_dir/run-id"
work_dir=$(mktemp -d "$artifact_dir/.work.XXXXXX")
cache_dir=${POS_TEST_CACHE_DIR:-$(dirname "$repo_dir")/cache}
mkdir -p "$cache_dir/npm"
cache_dir=$(cd "$cache_dir" && pwd)
cleanup() {
  local status=$?
  trap - EXIT INT TERM
  for name in "$run_id-prep" "$run_id-tests"; do
    if [[ $(docker inspect -f '{{index .Config.Labels "pos.integration.run"}}' "$name" 2>/dev/null || true) == "$run_id" ]]; then
      docker rm -f "$name" >/dev/null 2>&1 || true
    fi
  done
  rm -rf -- "$work_dir"
  printf 'exit_code=%s\n' "$status" > "$artifact_dir/result.txt"
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
tar -C "$repo_dir/ui" --exclude=node_modules --exclude=dist --exclude=.env --exclude='.env.*' -cf - . |
  tar -C "$work_dir" -xf -
image=mcr.microsoft.com/playwright:v1.47.2-jammy
docker image inspect "$image" >/dev/null
common=(--rm --label "pos.integration.run=$run_id" --user "$(id -u):$(id -g)"
  --cpus 1.5 --memory 1800m --memory-swap 1800m --pids-limit 512 --shm-size 256m
  -v "$work_dir:/src" -w /src -v "$artifact_dir:/artifacts"
  -v "$cache_dir/npm:/npm-cache" -e npm_config_cache=/npm-cache)
network=bridge
npm_mode=()
if $offline; then network=none; npm_mode+=(--offline); fi
docker run "${common[@]}" --name "$run_id-prep" --network "$network" "$image" \
  npm ci --no-audit --no-fund "${npm_mode[@]}" > "$artifact_dir/dependencies.log" 2>&1
docker run "${common[@]}" --name "$run_id-tests" --network none \
  -v "$modules_dir:/opt/browser/node_modules:ro" -e PLAYWRIGHT_MODULE=/opt/browser/node_modules/playwright \
  -e PRODUCT_KIND_TEST_URL=http://127.0.0.1:4189 -e PRODUCT_RECIPE_TEST_URL=http://127.0.0.1:4189 \
  "$image" bash -euo pipefail -c '
    # The browser image has an older Node20 build. Explicit ESM keeps its
    # loader compatible with Tailwind without changing application sources.
    cp vite.config.js vite.config.mjs
    npm run build -- --config vite.config.mjs
    node node_modules/vite/bin/vite.js --config vite.config.mjs --host 127.0.0.1 --port 4189 --strictPort > /artifacts/vite.log 2>&1 &
    vite_pid=$!
    trap "kill $vite_pid 2>/dev/null || true" EXIT
    ready=false
    for ((attempt=0; attempt<60; attempt++)); do
      if curl -fsS http://127.0.0.1:4189/tests/fixtures/product-kinds.html >/dev/null; then ready=true; break; fi
      sleep 1
    done
    $ready
    node tests/product-kinds.browser.cjs
    node tests/product-recipes.browser.cjs
    node tests/fnb-costing.browser.cjs
  ' > "$artifact_dir/browser.log" 2>&1
echo 'Product kinds, recipe selection and F&B browser checks passed.'
