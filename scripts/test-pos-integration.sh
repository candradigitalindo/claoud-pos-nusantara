#!/usr/bin/env bash
# Reproducible, synthetic-only integration checks. No host ports, production
# volumes, .env files or Docker Compose project are used. Tool downloads happen
# before tests; test containers use a new internal network (or network=none).
set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
staging_dir=$(dirname "$repo_dir")
phase=all
flutter_dir="${POS_TEST_FLUTTER_DIR:-$staging_dir/flutter}"
flutter_sdk="${POS_TEST_FLUTTER_SDK:-$staging_dir/toolchain/flutter-sdk}"
cache_dir="${POS_TEST_CACHE_DIR:-$staging_dir/cache}"
artifact_dir=""
offline=false

usage() {
  cat <<'USAGE'
Usage: scripts/test-pos-integration.sh [options]
  --phase all|backend|frontend|flutter  Run selected checks (default all).
  --artifacts DIR                      New output directory for logs and reports.
  --flutter DIR                        Flutter source (default ../flutter).
  --flutter-sdk DIR                    Flutter 3.41.9 checkout/cache.
  --offline                            Reuse cached images/dependencies only.

Requires Docker, Bash, Git, tar and Python 3. Every backend/flutter run creates its own
internal Docker network and tmpfs PostgreSQL *_test database. The server runs
only for migration bootstrap and Flutter E2E, never concurrently with Go tests.
QRIS is explicitly simulated. Resource cleanup is restricted to this run's IDs.
USAGE
}

while (($#)); do
  case "$1" in
    --phase|--artifacts|--flutter|--flutter-sdk)
      (($# >= 2)) || { usage >&2; exit 2; }
      case "$1" in
        --phase) phase=$2;; --artifacts) artifact_dir=$2;;
        --flutter) flutter_dir=$2;; --flutter-sdk) flutter_sdk=$2;;
      esac
      shift 2;;
    --offline) offline=true; shift;;
    -h|--help) usage; exit 0;;
    *) usage >&2; exit 2;;
  esac
done
case "$phase" in all|backend|frontend|flutter) ;; *) usage >&2; exit 2;; esac
for tool in docker git tar python3; do command -v "$tool" >/dev/null || { echo "Missing $tool" >&2; exit 2; }; done
docker info >/dev/null

run_id="pos-it-$(date -u +%Y%m%d%H%M%S)-$$-$RANDOM"
if [[ -z $artifact_dir ]]; then artifact_dir="$staging_dir/artifacts/$run_id"; fi
mkdir -p "$artifact_dir"
artifact_dir=$(cd "$artifact_dir" && pwd)
if [[ -e "$artifact_dir/run-id" ]]; then echo "Artifact directory already contains a run" >&2; exit 2; fi
printf '%s\n' "$run_id" > "$artifact_dir/run-id"
work_dir=$(mktemp -d "$artifact_dir/.work.XXXXXX")
mkdir -p "$cache_dir/go-mod" "$cache_dir/go-build" "$cache_dir/npm" "$cache_dir/flutter"
cache_dir=$(cd "$cache_dir" && pwd)
containers=()
network_id=""
api_id=""
db_id=""
step=0
owner="$(id -u):$(id -g)"
db_name=pos_integration_test
test_network=none
dependency_network=bridge
$offline && dependency_network=none

cleanup() {
  local exit_status=$?
  trap - EXIT INT TERM
  [[ -z $api_id ]] || docker logs "$api_id" > "$artifact_dir/api.log" 2>&1 || true
  [[ -z $db_id ]] || docker logs "$db_id" > "$artifact_dir/postgres.log" 2>&1 || true
  for cid in "${containers[@]}"; do
    if [[ $(docker inspect -f '{{index .Config.Labels "pos.integration.run"}}' "$cid" 2>/dev/null || true) == "$run_id" ]]; then
      docker rm -f "$cid" >/dev/null 2>&1 || true
    fi
  done
  if [[ -n $network_id ]] && [[ $(docker network inspect -f '{{index .Labels "pos.integration.run"}}' "$network_id" 2>/dev/null || true) == "$run_id" ]]; then
    docker network rm "$network_id" >/dev/null 2>&1 || true
  fi
  rm -rf -- "$work_dir"
  printf 'phase=%s\nexit_code=%s\n' "$phase" "$exit_status" > "$artifact_dir/result.txt"
  printf 'Integration artifacts: %s\n' "$artifact_dir"
  exit "$exit_status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

snapshot() {
  local source=$1 target=$2
  mkdir -p "$target"
  tar -C "$source" --exclude=.git --exclude=.env --exclude='.env.*' \
    --exclude=node_modules --exclude=.dart_tool --exclude=build --exclude=dist \
    --exclude=uploads --exclude=private --exclude=artifacts --exclude=coverage \
    --exclude='*.apk' --exclude='*.aab' -cf - . | tar -C "$target" -xf -
}
snapshot "$repo_dir" "$work_dir/cloud"

# Creating before starting means the cleanup list contains only IDs that this
# invocation really created. Name collisions cannot delete somebody else's run.
run_logged() {
  local label=$1 cid code
  shift
  step=$((step + 1))
  printf 'Running %s\n' "$label"
  cid=$(docker create --pull=never --label "pos.integration.run=$run_id" --name "$run_id-$step" "$@")
  containers+=("$cid")
  docker start -a "$cid" 2>&1 | tee "$artifact_dir/$label.log" || true
  code=$(docker inspect -f '{{.State.ExitCode}}' "$cid")
  if [[ $code != 0 ]]; then echo "$label failed with exit $code" >&2; return "$code"; fi
}

required_images=(golang:1.24-alpine)
[[ $phase == frontend ]] || required_images+=(postgres:16-alpine)
[[ $phase != all && $phase != frontend ]] || required_images+=(node:20-alpine)
for image in "${required_images[@]}"; do
  if ! docker image inspect "$image" >/dev/null 2>&1; then
    $offline && { echo "Image missing in offline mode: $image" >&2; exit 2; }
    docker pull "$image"
  fi
done
# Previous manual preparation may have used root. Only dedicated test caches are
# adjusted so tool containers can run as the invoking user and leave cleanable files.
run_logged cache-permissions --network none --cpus 1 --memory 256m --memory-swap 256m \
  -v "$cache_dir:/test-cache" golang:1.24-alpine chown -R "$owner" /test-cache

go_args=(--user "$owner" --cpus 1 --memory 1800m --memory-swap 1800m --pids-limit 512
  -v "$work_dir/cloud:/src" -w /src -v "$cache_dir/go-mod:/go/pkg/mod" -v "$cache_dir/go-build:/cache/build"
  -v "$artifact_dir:/artifacts" -e GOMODCACHE=/go/pkg/mod -e GOCACHE=/cache/build -e CGO_ENABLED=0
  -e GOMAXPROCS=1 -e GOGC=20 -e GOMEMLIMIT=950MiB -e GOTOOLCHAIN=local)
db_env=(-e DB_HOST=db -e DB_PORT=5432 -e DB_NAME="$db_name" -e DB_USER=postgres
  -e DB_PASSWORD=pos-integration-only -e DB_SSLMODE=disable -e TZ=Asia/Jakarta
  -e JWT_SECRET=pos-integration-synthetic-jwt-2026-0123456789 -e ADMIN_TOKEN=
  -e WA_GATEWAY_URL= -e ANALYTICS_URL= -e NARRATIVE_PROVIDER= -e GO2RTC_URL=
  -e PAYMENT_GATEWAY_PROVIDER=mock -e PAYMENT_GATEWAY_ALLOW_MOCK=true
  -e QRIS_WEBHOOK_SECRET=pos-integration-simulation-only -e RATE_LIMIT_PER_MINUTE=10000)

if [[ $phase != frontend ]]; then
  if $offline; then
    go_args+=(-e GOPROXY=off -e GOSUMDB=off)
  fi
  run_logged go-dependencies --network "$dependency_network" "${go_args[@]}" golang:1.24-alpine go mod download
  run_logged go-build --network none "${go_args[@]}" -e CGO_ENABLED=0 golang:1.24-alpine \
    go build -p 1 -o /artifacts/cloud-api .
fi

if [[ $phase == all || $phase == frontend ]]; then
  npm_mode=()
  $offline && npm_mode+=(--offline)
  run_logged ui-dependencies --network "$dependency_network" --user "$owner" --cpus 1 --memory 1200m --memory-swap 1200m \
    -v "$work_dir/cloud/ui:/src" -w /src -v "$cache_dir/npm:/npm-cache" -e npm_config_cache=/npm-cache \
    node:20-alpine npm ci --no-audit --no-fund "${npm_mode[@]}"
  run_logged ui-build --network none --user "$owner" --cpus 1 --memory 1500m --memory-swap 1500m \
    -v "$work_dir/cloud/ui:/src" -w /src -e NODE_OPTIONS=--max-old-space-size=1100 \
    node:20-alpine npm run build
  tar -C "$work_dir/cloud/ui" -czf "$artifact_dir/ui-dist.tar.gz" dist
fi

if [[ $phase == all || $phase == flutter ]]; then
  [[ -f "$flutter_dir/pubspec.yaml" ]] || { echo "Flutter source not found: $flutter_dir" >&2; exit 2; }
  if [[ ! -x "$flutter_sdk/bin/flutter" ]]; then
    $offline && { echo "Flutter SDK missing in offline mode" >&2; exit 2; }
    mkdir -p "$(dirname "$flutter_sdk")"
    git clone --depth 1 --branch 3.41.9 https://github.com/flutter/flutter.git "$flutter_sdk"
  fi
  flutter_sdk=$(cd "$flutter_sdk" && pwd)
  if ! docker image inspect pos-integration-flutter-tools:local >/dev/null 2>&1; then
    $offline && { echo "Flutter tooling image missing in offline mode" >&2; exit 2; }
    docker build --memory 1500m -f "$repo_dir/scripts/Dockerfile.flutter-test" \
      -t pos-integration-flutter-tools:local "$repo_dir/scripts" | tee "$artifact_dir/flutter-image.log"
  fi
  snapshot "$flutter_dir" "$work_dir/flutter"
  mkdir -p "$flutter_sdk/bin/cache"
  run_logged flutter-cache-permissions --network none --cpus 1 --memory 256m --memory-swap 256m \
    -v "$flutter_sdk/bin/cache:/sdk-cache" golang:1.24-alpine chown -R "$owner" /sdk-cache
  flutter_args=(--user "$owner" --cpus 1.5 --memory 1500m --memory-swap 1500m --pids-limit 512
    -v "$flutter_sdk:/opt/flutter" -v "$cache_dir/flutter:/cache" -v "$work_dir/flutter:/src" -w /src
    -e GIT_CONFIG_GLOBAL=/cache/gitconfig -e TZ=Asia/Jakarta -e FLUTTER_SUPPRESS_ANALYTICS=true
    -e DART_SUPPRESS_ANALYTICS=true -e CI=true)
  pub_mode=""
  $offline && pub_mode=--offline
  run_logged flutter-dependencies --network "$dependency_network" "${flutter_args[@]}" \
    pos-integration-flutter-tools:local bash -euc '
      cd /tmp
      git config --global --add safe.directory /opt/flutter
      flutter --version
      flutter precache --linux
      cd /src
      flutter pub get "$@"
    ' prepare ${pub_mode:+"$pub_mode"}

  # sqlite3's native build hook downloads a hash-verified library on the first
  # test compilation. Its directory hash is process-dependent, so preserve the
  # whole hook-runner metadata rather than guessing a download-* directory.
  sdk_revision=$(git -c safe.directory="$flutter_sdk" -C "$flutter_sdk" rev-parse HEAD)
  hook_key=$(python3 - "$work_dir/flutter/pubspec.lock" "$sdk_revision" "$(uname -m)" <<'PY'
import hashlib, pathlib, sys
digest = hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes())
digest.update(('\n' + sys.argv[2] + '\n' + sys.argv[3]).encode())
print(digest.hexdigest())
PY
)
  hook_cache="$cache_dir/flutter/native-hooks/$hook_key"
  hook_source=""
  if [[ -d "$hook_cache/hooks_runner" ]]; then
    hook_source="$hook_cache/hooks_runner"
  elif [[ -d "$flutter_dir/.dart_tool/hooks_runner" ]]; then
    hook_source="$flutter_dir/.dart_tool/hooks_runner"
  fi
  if [[ -n $hook_source ]]; then
    mkdir -p "$work_dir/flutter/.dart_tool/hooks_runner"
    tar -C "$hook_source" --exclude='*.tmp' -cf - . | tar -C "$work_dir/flutter/.dart_tool/hooks_runner" -xf -
  elif $offline; then
    echo "Offline native-asset cache missing. Run Flutter preparation online once." >&2
    exit 2
  fi
  if ! run_logged flutter-native-warmup --network "$dependency_network" "${flutter_args[@]}" \
      pos-integration-flutter-tools:local flutter test --no-pub --concurrency=1 \
      --reporter=json test/charge_math_test.dart; then
    echo "Native-asset preparation failed; offline runs require a complete verified SQLite hook cache." >&2
    exit 1
  fi
  mkdir -p "$hook_cache/hooks_runner"
  tar -C "$work_dir/flutter/.dart_tool/hooks_runner" --exclude='*.tmp' -cf - . | tar -C "$hook_cache/hooks_runner" -xf -
  run_logged flutter-analyze --network none "${flutter_args[@]}" \
    pos-integration-flutter-tools:local flutter analyze --no-pub --no-fatal-infos --no-fatal-warnings
fi

if [[ $phase == frontend ]]; then exit 0; fi

network_id=$(docker network create --internal --label "pos.integration.run=$run_id" "$run_id")
test_network=$network_id
db_id=$(docker create --pull=never --label "pos.integration.run=$run_id" --name "$run_id-db" \
  --network "$test_network" --network-alias db --cpus 1 --memory 768m --memory-swap 768m --pids-limit 256 \
  --tmpfs /var/lib/postgresql/data:rw,nosuid,size=512m \
  -e POSTGRES_DB="$db_name" -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=pos-integration-only \
  postgres:16-alpine -c max_connections=40 -c shared_buffers=64MB)
containers+=("$db_id")
docker start "$db_id" >/dev/null
ready=false
for ((i=0; i<60; i++)); do
  if docker exec "$db_id" pg_isready -U postgres -d "$db_name" >/dev/null 2>&1; then ready=true; break; fi
  sleep 1
done
$ready || { echo "Synthetic PostgreSQL did not become ready" >&2; exit 1; }

api_id=$(docker create --pull=never --label "pos.integration.run=$run_id" --name "$run_id-api" \
  --network "$test_network" --network-alias api --user "$owner" --cpus 1 --memory 512m --memory-swap 512m \
  -v "$artifact_dir/cloud-api:/test/cloud-api:ro" --tmpfs /app:rw,mode=1777 -w /app \
  "${db_env[@]}" -e PORT=4000 golang:1.24-alpine /test/cloud-api)
containers+=("$api_id")
start_api() {
  docker start "$api_id" >/dev/null
  local ready=false
  for ((i=0; i<90; i++)); do
    if docker exec "$api_id" wget -q -O /dev/null http://127.0.0.1:4000/api/v1/ping 2>/dev/null; then ready=true; break; fi
    [[ $(docker inspect -f '{{.State.Running}}' "$api_id") == true ]] || break
    sleep 1
  done
  $ready || { echo "Synthetic API did not become ready" >&2; docker logs "$api_id"; return 1; }
}
start_api
docker stop -t 10 "$api_id" >/dev/null

if [[ $phase == all || $phase == backend ]]; then
  run_logged go-tests --network "$test_network" "${go_args[@]}" "${db_env[@]}" \
    golang:1.24-alpine go test -json -count=1 -p 1 -parallel 1 -timeout 20m ./...
fi

if [[ $phase == all || $phase == flutter ]]; then
  # The E2E data is intentionally separate from any business seed or backup.
  docker exec -i "$db_id" psql -v ON_ERROR_STOP=1 -U postgres -d "$db_name" \
    < "$repo_dir/scripts/pos-integration-seed.sql" | tee "$artifact_dir/seed.log"
  start_api
  run_logged flutter-tests --network "$test_network" "${flutter_args[@]}" \
    -e E2E_CLOUD_URL=http://api:4000 -e E2E_SLUG=e2e-uji \
    -e E2E_OUTLET_ID=01E2ETESTOUTLET00000000001 -e E2E_API_KEY=e2e-api-key \
    -e E2E_PRODUCT_ID=01E2ETESTPRODUK00000000001 -e E2E_ADDON_ID=01E2ETESTADDON00000000001A \
    -e E2E_RESTORE_OUTLET_ID=01E2ERESTOREOUTLET00000001 -e E2E_RESTORE_API_KEY=e2e-restore-key \
    -e E2E_PHOTO_OUTLET_ID=01E2EPHOTOOUTLET000000001A -e E2E_PHOTO_API_KEY=e2e-photo-key \
    -e E2E_PHOTO_PRODUCT_ID=01E2EPHOTOPRODUCT00000001A \
    -e E2E_PHOTO_ADMIN_USER=e2e-photo-admin -e E2E_PHOTO_ADMIN_PASSWORD=Photo-E2E-Synthetic-2026! \
    pos-integration-flutter-tools:local flutter test --concurrency=1 --no-pub --reporter=json
fi

python3 - "$artifact_dir" "$phase" <<'PY'
import collections, json, pathlib, sys
out = pathlib.Path(sys.argv[1])
phase = sys.argv[2]
summary = {}
def events(name):
    for line in (out / name).read_text().splitlines():
        try: yield json.loads(line)
        except json.JSONDecodeError: continue
if phase in ('all', 'backend'):
    results = [e for e in events('go-tests.log') if e.get('Test') and e.get('Action') in ('pass', 'skip', 'fail')]
    summary['go'] = dict(collections.Counter(e['Action'] for e in results))
    passed = {e['Test'] for e in results if e['Action'] == 'pass'}
    required = {'TestDBFlutterMastersFeedOnlineOrdersWithoutChangingReservationIDs',
                'TestDBQRISPaymentAndOnlinePromotionRollbackTogether',
                'TestPOSMergedRoutesPreserveBothFeatureFamilies',
                'TestProductPhotoUploadReplaceDeleteSyncAndPOSOwnership',
                'TestProductPhotoRejectsInvalidImageAndOtherOutlet',
                'TestProductPhotoDatabaseFailureCleansNewFileAndKeepsPrevious',
                'TestProductPhotoCleanupStaysWithinOwnedFiles',
                'TestProductPhotoGETUsesCurrentFileAndRejectsTraversal',
                'TestProductKindEffectiveViewAndScopedFilters',
                'TestProductKindEditsPreserveFnbCostsAndIgnorePOSClassification',
                'TestProductKindFnbEditPublishesUpdatesAndActivityIsOutsideCosting',
                'TestProductKindMigrationIsIdempotentAndKeepsExistingProfile',
                'TestProductKindRecipeFallbackChangesPublishUpdates',
                'TestProductKindRecipeConcurrentLinkRequiresRetry',
                'TestProductClassificationPreviewScopedCompleteAndConservative',
                'TestProductClassificationApplyPreservesCostsAndAudit',
                'TestProductClassificationApplyRejectsTamperingAndStaleAtomically',
                'TestProductClassificationConcurrentApplyHasSingleWinner',
                'TestProductClassificationATVOverlandPolicyApplyAndRename',
                'TestProductClassificationRoutesRequireUpdatePermissionToApply'}
    if not required <= passed:
        raise SystemExit('Required integration tests did not pass (possibly skipped): ' + ', '.join(sorted(required-passed)))
if phase in ('all', 'flutter'):
    suites, tests, results, passed_suites = {}, {}, [], set()
    for e in events('flutter-tests.log'):
        if e.get('type') == 'suite': suites[e['suite']['id']] = e['suite'].get('path', '')
        if e.get('type') == 'testStart': tests[e['test']['id']] = e['test']['suiteID']
        if e.get('type') == 'testDone' and not e.get('hidden'):
            results.append('skip' if e.get('skipped') else e.get('result'))
            if e.get('result') == 'success' and not e.get('skipped'):
                passed_suites.add(suites.get(tests.get(e['testID']), ''))
    summary['flutter'] = dict(collections.Counter(results))
    for name in ('online_order_e2e_test.dart', 'reservation_e2e_test.dart', 'restore_e2e_test.dart', 'photo_sync_e2e_test.dart'):
        if not any(path.endswith(name) for path in passed_suites):
            raise SystemExit('Required Flutter E2E suite did not run successfully: ' + name)
(out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
PY

printf 'All requested integration checks passed.\n'
