# shellcheck shell=bash
# Pustaka bersama deploy/demo.sh dan deploy/release.sh (di-source, bukan dijalankan).
#
# Semua nama container, proyek compose, jaringan, dan lokasi arsip ada di sini
# saja. Ubah di sini, bukan di masing-masing skrip.

set -Eeuo pipefail
umask 077

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# Arsip rilis & keadaan demo hidup DI LUAR repo: berisi rahasia dan cadangan
# basis data, tidak boleh ikut ter-commit atau masuk konteks build Docker.
STAGING_DIR="${NBP_STAGING_DIR:-$(cd "$REPO_DIR/.." && pwd)/pos-integration-staging}"
RELEASES_DIR="${NBP_RELEASES_DIR:-$STAGING_DIR/releases}"
DEMO_DIR="${NBP_DEMO_DIR:-$STAGING_DIR/demo}"

# ── Produksi: docker-compose.yml di akar repo, proyek "cloud-pos" ─────────────
PROD_COMPOSE="$REPO_DIR/docker-compose.yml"
PROD_PROJECT=cloud-pos
PROD_DB=cloud-pos-db-1
PROD_APP=cloud-pos-app-1
PROD_WEB=cloud-pos-nginx-1
PROD_DB_NAME=cloud_pos
PROD_HOST=pos.nbp.co.id

# ── Demo: deploy/demo/compose.yml, proyek "cloud-pos-demo" ────────────────────
DEMO_COMPOSE="$REPO_DIR/deploy/demo/compose.yml"
DEMO_PROJECT=cloud-pos-demo
DEMO_DB=cloud-pos-demo-db-1
DEMO_APP=cloud-pos-demo-app-1
DEMO_WEB=cloud-pos-demo-web-1
DEMO_DB_NAME=cloud_pos_demo
DEMO_HOST=demo.nbp.co.id

# Jaringan docker bersama yang HANYA berisi nginx produksi dan web demo.
# --internal: tidak ada jalan keluar ke internet lewat jaringan ini.
EDGE_NETWORK=nbp-edge

APP_REPO=cloud-pos-app
WEB_REPO=cloud-pos-nginx

# ── Keluaran ──────────────────────────────────────────────────────────────────
if [[ -t 1 ]]; then
  _R=$'\e[31m' _G=$'\e[32m' _Y=$'\e[33m' _C=$'\e[36m' _B=$'\e[1m' _N=$'\e[0m'
else
  _R='' _G='' _Y='' _C='' _B='' _N=''
fi
info() { printf '%s==>%s %s\n' "$_C" "$_N" "$*"; }
ok()   { printf '%s ok%s  %s\n' "$_G" "$_N" "$*"; }
warn() { printf '%s !!%s  %s\n' "$_Y" "$_N" "$*" >&2; }
die()  { printf '%sGAGAL:%s %s\n' "$_R" "$_N" "$*" >&2; exit 1; }
bold() { printf '%s%s%s' "$_B" "$*" "$_N"; }

now_ts()  { date +%Y%m%d-%H%M%S; }
now_iso() { date --iso-8601=seconds; }

# Satu proses deploy/demo pada satu waktu: reset demo dan promosi produksi
# yang berjalan bersamaan saling berebut pg_dump, image, dan nginx.
take_lock() {
  # Sudah dipegang proses ini atau induknya (fd 9 ikut diwariskan ke anak).
  [[ -n "${NBP_DEPLOY_LOCK:-}" ]] && return 0
  mkdir -p "$STAGING_DIR"
  exec 9>"$STAGING_DIR/.deploy.lock"
  flock -n 9 || die "Ada proses deploy/demo lain yang sedang berjalan (kunci: $STAGING_DIR/.deploy.lock)."
  export NBP_DEPLOY_LOCK=1
}

require_tools() {
  local t
  for t in docker jq curl flock; do
    command -v "$t" >/dev/null 2>&1 || die "Perintah '$t' tidak ditemukan."
  done
  docker info >/dev/null 2>&1 || die "Docker tidak bisa diakses (apakah user ini anggota grup docker?)."
}

ensure_edge_network() {
  if ! docker network inspect "$EDGE_NETWORK" >/dev/null 2>&1; then
    info "Membuat jaringan $EDGE_NETWORK (internal; penghubung nginx produksi ↔ web demo)"
    docker network create --internal "$EDGE_NETWORK" >/dev/null
  fi
}

image_id()           { docker image inspect -f '{{.Id}}' "$1" 2>/dev/null || true; }
container_image_id() { docker inspect -f '{{.Image}}' "$1" 2>/dev/null || true; }
container_running()  { [[ "$(docker inspect -f '{{.State.Running}}' "$1" 2>/dev/null || true)" == true ]]; }
container_restarts() { docker inspect -f '{{.RestartCount}}' "$1" 2>/dev/null || echo 0; }

# Nama rilis = nama folder di $RELEASES_DIR (contoh: 20261123-140501-rekap-darurat).
valid_release_id() { [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{2,100}$ ]]; }

# build.json buatan release.sh (folder rilis lama punya build.json berformat lain).
is_release_record() {
  jq -e '(.images.app.tag | type) == "string" and (.images.web.tag | type) == "string" and (.source.mode | type) == "string"' \
    "$1" >/dev/null 2>&1
}

release_dir() {
  valid_release_id "$1" || die "Nama rilis tidak sah: '$1'"
  local d="$RELEASES_DIR/$1"
  [[ -f "$d/build.json" ]] && is_release_record "$d/build.json" \
    || die "Rilis '$1' tidak ditemukan atau bukan rilis buatan release.sh. Lihat: deploy/release.sh list"
  printf '%s' "$d"
}

# Menunggu API menjawab /api/v1/ping dari DALAM container (tanpa port publik).
wait_ping() {
  local c="$1" limit="${2:-180}" waited=0
  while (( waited < limit )); do
    if docker exec "$c" wget -qO- -T 3 http://127.0.0.1:4000/api/v1/ping >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
    waited=$((waited + 2))
  done
  return 1
}

utc_stamp() { date -u +%Y-%m-%dT%H:%M:%SZ; }

# Sehat = ping menjawab, tidak ada restart selama masa tenang 8 detik, ping
# kedua tetap menjawab, dan tidak ada "panic:" di log sejak $since (utc_stamp
# yang diambil sebelum container dinyalakan). Ringkasan ditulis ke stdout.
check_app_health() {
  local c="$1" limit="${2:-180}" since="${3:-}"
  if ! wait_ping "$c" "$limit"; then
    echo "API tidak menjawab /api/v1/ping dalam ${limit} detik"
    return 1
  fi
  local r1 r2 logs panics migfail again=ok
  r1=$(container_restarts "$c")
  sleep 8
  r2=$(container_restarts "$c")
  docker exec "$c" wget -qO- -T 3 http://127.0.0.1:4000/api/v1/ping >/dev/null 2>&1 || again=gagal
  if [[ -n "$since" ]]; then logs=$(docker logs --since "$since" "$c" 2>&1); else logs=$(docker logs "$c" 2>&1); fi
  panics=$(grep -c 'panic:' <<<"$logs" || true)
  migfail=$(grep -ci 'migration failed' <<<"$logs" || true)
  echo "ping=ok/$again restart_saat_uji=$((r2 - r1)) panic=$panics migration_failed=$migfail"
  [[ "$again" == ok && "$r1" == "$r2" && "$panics" == 0 ]]
}

# Ketik ulang teks yang diminta untuk melanjutkan (bukan sekadar y/n).
confirm_typed() {
  local prompt="$1" expected="$2" answer
  [[ -t 0 ]] || die "Butuh konfirmasi interaktif. Jalankan dari terminal, atau tambahkan --yes."
  read -r -p "$prompt " answer
  [[ "$answer" == "$expected" ]] || die "Dibatalkan (masukan tidak cocok)."
}
