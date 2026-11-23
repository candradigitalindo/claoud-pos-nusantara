#!/usr/bin/env bash
# Lingkungan DEMO (demo.nbp.co.id): salinan produksi yang disanitasi, untuk
# latihan pengguna dan uji rilis sebelum naik ke produksi.
# Panduan lengkap: docs/lingkungan-demo.md
#
#   deploy/demo.sh init                       pasang pertama kali (rilis = image produksi saat ini)
#   deploy/demo.sh reset                      segarkan data dari produksi (disanitasi)
#   deploy/demo.sh deploy <rilis> [--reset]   jalankan rilis tertentu di demo
#   deploy/demo.sh status                     rilis, kesehatan, umur data, jalur domain
#   deploy/demo.sh logs [app|web|db|analytics]
#   deploy/demo.sh start | stop               nyalakan / matikan demo (data tetap)
#   deploy/demo.sh route-on                   sambungkan demo.nbp.co.id di nginx produksi (tanpa downtime)
#   deploy/demo.sh route-check                uji jalur demo.nbp.co.id dan pos.nbp.co.id
#   deploy/demo.sh outlet-keys                kunci API outlet demo (untuk tablet latihan)
#   deploy/demo.sh training-users             terapkan akun latihan ($DEMO_DIR/training-users.csv) sekarang
#   deploy/demo.sh psql                       konsol SQL basis data demo
#   deploy/demo.sh destroy                    hapus demo beserta datanya (produksi tidak tersentuh)

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

DEMO_ENV="$DEMO_DIR/demo.env"            # rahasia demo (dibuat sekali oleh init)
DEMO_STATE="$DEMO_DIR/state.env"         # rilis yang sedang berjalan di demo
DEMO_STATUS="$DEMO_DIR/status.json"      # catatan reset & deploy terakhir
SNAP_DIR="$DEMO_DIR/snapshots"           # salinan pg_dump produksi (2 terakhir)
TRAINING_CSV="$DEMO_DIR/training-users.csv"
EDGE_MARK_BEGIN='# >>> demo.nbp.co.id'
EDGE_MARK_END='# <<< demo.nbp.co.id'

usage() { sed -n '2,/^$/{s/^# \{0,1\}//;p}' "$0"; }

dc() { docker compose -p "$DEMO_PROJECT" -f "$DEMO_COMPOSE" "$@"; }

load_env() {
  [[ -f "$DEMO_ENV" ]] || die "Demo belum dipasang. Jalankan dulu: deploy/demo.sh init"
  set -a
  # shellcheck disable=SC1090
  . "$DEMO_ENV"
  # shellcheck disable=SC1090
  [[ -f "$DEMO_STATE" ]] && . "$DEMO_STATE"
  set +a
  export DEMO_NGINX_CONF="$DEMO_DIR/nginx.conf"
}

# Ditulis DI TEMPAT (inode tetap): bind mount berkas tunggal menempel pada
# inode, jadi berkas yang diganti (install/mv) tidak terlihat oleh container
# yang sedang jalan. Bila isinya berubah dan web demo menyala, nginx di-reload.
install_nginx_conf() {
  local src="$REPO_DIR/deploy/demo/nginx.conf" dst="$DEMO_DIR/nginx.conf"
  if [[ -f "$dst" ]] && cmp -s "$src" "$dst"; then return 0; fi
  cat "$src" > "$dst"
  chmod 644 "$dst"
  if container_running "$DEMO_WEB"; then
    if docker exec "$DEMO_WEB" nginx -t >/dev/null 2>&1; then
      docker exec "$DEMO_WEB" nginx -s reload >/dev/null 2>&1 && ok "konfigurasi web demo diperbarui"
    else
      docker exec "$DEMO_WEB" nginx -t >&2 || true
      die "Konfigurasi deploy/demo/nginx.conf ditolak nginx -t."
    fi
  fi
}

status_set() { # jalur-jq nilai-json
  local tmp; tmp=$(mktemp "$DEMO_DIR/.status.XXXXXX")
  if [[ -f "$DEMO_STATUS" ]]; then
    jq --argjson v "$2" "$1 = \$v" "$DEMO_STATUS" > "$tmp"
  else
    jq -n --argjson v "$2" "$1 = \$v" > "$tmp"
  fi
  mv "$tmp" "$DEMO_STATUS"
}

wait_db_healthy() {
  local i
  for i in $(seq 1 60); do
    [[ "$(docker inspect -f '{{.State.Health.Status}}' "$DEMO_DB" 2>/dev/null || true)" == healthy ]] && return 0
    sleep 2
  done
  die "Basis data demo tidak sehat dalam 120 detik."
}

prod_web_on_edge() {
  docker inspect -f '{{json .NetworkSettings.Networks}}' "$PROD_WEB" 2>/dev/null \
    | jq -e --arg n "$EDGE_NETWORK" 'has($n)' >/dev/null 2>&1
}

prod_web_has_route() {
  docker exec "$PROD_WEB" grep -q "server_name $DEMO_HOST;" /etc/nginx/conf.d/default.conf 2>/dev/null
}

# ── reset: produksi → demo (disanitasi) ──────────────────────────────────────
do_reset() {
  container_running "$PROD_DB" || die "Basis data produksi ($PROD_DB) tidak berjalan."
  mkdir -p "$SNAP_DIR"
  local ts dump started
  ts=$(now_ts); started=$(now_iso)
  dump="$SNAP_DIR/prod-$ts.dump"

  info "1/6 Menyalin basis data produksi (pg_dump hanya-baca; sesi & antrean WhatsApp tidak ikut)"
  docker exec "$PROD_DB" pg_dump -Fc -U postgres -d "$PROD_DB_NAME" \
    --exclude-table-data='whatsmeow_*' \
    --exclude-table-data=wa_messages --exclude-table-data=wa_gateway_events \
    --exclude-table-data=wa_recipients --exclude-table-data=wa_broadcasts \
    > "$dump.part"
  mv "$dump.part" "$dump"
  ok "$(du -h "$dump" | cut -f1)  $dump"
  ls -1t "$SNAP_DIR"/prod-*.dump 2>/dev/null | tail -n +3 | xargs -r rm -f

  info "2/6 Menghentikan app demo, menyiapkan basis data demo kosong"
  dc create --no-recreate >/dev/null 2>&1 || dc create --no-recreate
  dc stop app >/dev/null 2>&1 || true
  dc start db >/dev/null
  wait_db_healthy
  docker exec "$DEMO_DB" psql -U postgres -d postgres -v ON_ERROR_STOP=1 -q \
    -c "DROP DATABASE IF EXISTS $DEMO_DB_NAME WITH (FORCE)" \
    -c "CREATE DATABASE $DEMO_DB_NAME" >/dev/null

  info "3/6 Memulihkan salinan ke basis data demo"
  local rlog="$SNAP_DIR/restore-$ts.log"
  docker exec -i "$DEMO_DB" pg_restore -U postgres -d "$DEMO_DB_NAME" --no-owner --no-privileges \
    < "$dump" > "$rlog" 2>&1 || true
  local rerr; rerr=$(grep -ci 'error' "$rlog" || true)
  if [[ "$rerr" != 0 ]]; then
    sed -n '1,15p' "$rlog" >&2
    die "pg_restore melaporkan $rerr baris error (log: $rlog). App demo dibiarkan mati."
  fi
  ok "dipulihkan tanpa error"

  info "4/6 Sanitasi: kunci API, WhatsApp, CCTV, medsos, Drive, nomor HP pelanggan, sesi"
  docker exec -i "$DEMO_DB" psql -U postgres -d "$DEMO_DB_NAME" -q -At \
    < "$REPO_DIR/deploy/demo/sanitize.sql" | sed 's/^/     /'
  apply_training_users

  info "5/6 Menyalin berkas unggahan & dokumen (tanpa APK di uploads/apps)"
  docker run --rm --network none \
    -v "${PROD_PROJECT}_uploads_data:/src/uploads:ro" \
    -v "${PROD_PROJECT}_documents_data:/src/documents:ro" \
    -v "${DEMO_PROJECT}_uploads:/dst/uploads" \
    -v "${DEMO_PROJECT}_documents:/dst/documents" \
    alpine:3.20 sh -euc '
      for d in uploads documents; do
        find "/dst/$d" -mindepth 1 -delete
        (cd "/src/$d" && tar --exclude=./apps -cf - .) | tar -xf - -C "/dst/$d"
      done
      echo "     unggahan $(du -sh /dst/uploads | cut -f1), dokumen $(du -sh /dst/documents | cut -f1)"'

  info "6/6 Menyalakan demo (migrasi rilis demo berjalan di salinan ini)"
  DEMO_UP_SINCE=$(utc_stamp)
  dc up -d >/dev/null 2>&1 || dc up -d

  status_set '.data' "$(jq -n --arg at "$started" --arg dump "$dump" \
    '{snapshot_at: $at, snapshot_file: $dump, source: "produksi (cloud-pos-db-1), disanitasi"}')"
}

apply_training_users() {
  [[ -f "$TRAINING_CSV" ]] || return 0
  info "     akun latihan dari $TRAINING_CSV"
  python3 -I - "$TRAINING_CSV" <<'PY' | docker exec -i "$DEMO_DB" psql -U postgres -d "$DEMO_DB_NAME" -q -At | sed 's/^/     /'
import csv, sys
q = lambda s: "'" + s.replace("'", "''") + "'"
rows = []
with open(sys.argv[1], newline="", encoding="utf-8") as f:
    for r in csv.DictReader(f):
        u, n, role, pw = (r.get(k, "").strip() for k in ("username", "nama", "role", "password"))
        if u and role and pw:
            rows.append("(%s,%s,%s,%s)" % (q(u), q(n or u), q(role), q(pw)))
if not rows:
    sys.exit(0)
vals = ",\n".join(rows)
print(r"""\set ON_ERROR_STOP on
SELECT NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pgcrypto') AS dibuat \gset
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TEMP TABLE akun_latihan (username text, nama text, role text, pw text);
INSERT INTO akun_latihan VALUES
%s;
SELECT 'DILEWATI ' || username || ': role "' || role || '" tidak ada / terlalu panjang'
  FROM akun_latihan a
 WHERE NOT EXISTS (SELECT 1 FROM roles r WHERE r.name = a.role) OR length(a.role) > 20;
INSERT INTO cloud_admins (id, username, password_hash, name, role, is_active)
SELECT '01' || upper(substr(md5('nbp-demo:' || a.username), 1, 24)), a.username,
       crypt(a.pw, gen_salt('bf', 10)), a.nama, a.role, true
  FROM akun_latihan a
 WHERE EXISTS (SELECT 1 FROM roles r WHERE r.name = a.role) AND length(a.role) <= 20
ON CONFLICT (username) DO UPDATE
   SET password_hash = EXCLUDED.password_hash, name = EXCLUDED.name, role = EXCLUDED.role,
       is_active = true, token_version = cloud_admins.token_version + 1;
SELECT 'akun latihan aktif: ' || count(*) FROM cloud_admins c JOIN akun_latihan a USING (username);
\if :dibuat
DROP EXTENSION pgcrypto;
\endif""" % vals)
PY
}

# ── perintah ────────────────────────────────────────────────────────────────
cmd_init() {
  take_lock
  require_tools
  mkdir -p "$DEMO_DIR" "$SNAP_DIR"
  chmod 700 "$DEMO_DIR"
  if [[ ! -f "$DEMO_ENV" ]]; then
    info "Membuat rahasia demo → $DEMO_ENV"
    {
      echo "# Rahasia lingkungan DEMO, dibuat $(now_iso). Khusus demo — jangan dipakai di produksi."
      echo "DEMO_DB_PASSWORD=$(openssl rand -hex 24)"
      echo "DEMO_JWT_SECRET=$(openssl rand -hex 32)"
      echo "DEMO_WEBHOOK_SECRET=$(openssl rand -hex 32)"
      echo "DEMO_ADMIN_TOKEN=$(openssl rand -hex 32)"
      echo "DEMO_CAMERA_ENC_KEY=$(openssl rand -hex 32)"
      echo "# Simulasi QRIS untuk latihan di tablet demo (opsional):"
      echo "# DEMO_PAYMENT_GATEWAY_PROVIDER=mock"
      echo "# DEMO_PAYMENT_GATEWAY_ALLOW_MOCK=true"
    } > "$DEMO_ENV"
    chmod 600 "$DEMO_ENV"
  else
    ok "rahasia demo sudah ada ($DEMO_ENV)"
  fi
  ensure_edge_network

  if [[ -f "$DEMO_STATE" ]]; then
    load_env
    ok "demo sudah terpasang dengan rilis ${DEMO_RELEASE:-?}"
    cmd_status
    return 0
  fi

  local rel="${1:-}"
  if [[ -z "$rel" ]]; then
    info "Mencatat image yang sedang berjalan di produksi sebagai rilis dasar demo"
    rel=$("$REPO_DIR/deploy/release.sh" adopt-prod prod-baseline)
  fi
  cmd_deploy "$rel" --reset
  echo
  info "Langkah berikut:"
  echo "   1. deploy/demo.sh route-on     (sambungkan demo.nbp.co.id di nginx produksi, tanpa downtime)"
  echo "   2. Cloudflare → DNS: CNAME 'demo' → pos.nbp.co.id, Proxied (awan oranye)"
  echo "      Urutan ini penting: tanpa route-on, demo.nbp.co.id akan menampilkan PRODUKSI."
}

cmd_reset() {
  take_lock
  require_tools
  load_env
  [[ -n "${DEMO_RELEASE:-}" ]] || die "Belum ada rilis di demo. Jalankan: deploy/demo.sh init"
  install_nginx_conf
  do_reset
  local health
  if health=$(check_app_health "$DEMO_APP" 240 "$DEMO_UP_SINCE"); then
    ok "demo sehat ($health), rilis $DEMO_RELEASE"
  else
    die "demo tidak sehat setelah reset: $health — lihat: deploy/demo.sh logs app"
  fi
}

cmd_deploy() {
  local rel="" reset=0 a
  for a in "$@"; do
    case "$a" in
      --reset) reset=1 ;;
      -*) die "Opsi tidak dikenal: $a" ;;
      *) rel="$a" ;;
    esac
  done
  [[ -n "$rel" ]] || die "Pakai: deploy/demo.sh deploy <rilis> [--reset]   (daftar: deploy/release.sh list)"
  take_lock
  require_tools
  load_env
  ensure_edge_network

  local d app_img web_img app_want web_want prev
  d=$(release_dir "$rel")
  app_img=$(jq -r '.images.app.tag' "$d/build.json")
  web_img=$(jq -r '.images.web.tag' "$d/build.json")
  app_want=$(jq -r '.images.app.id // empty' "$d/build.json")
  web_want=$(jq -r '.images.web.id // empty' "$d/build.json")
  [[ -n "$(image_id "$app_img")" ]] || die "Image $app_img tidak ada di mesin ini."
  [[ -n "$(image_id "$web_img")" ]] || die "Image $web_img tidak ada di mesin ini."
  [[ -z "$app_want" || "$(image_id "$app_img")" == "$app_want" ]] || die "Tag $app_img sudah menunjuk image lain dari yang dicatat rilis."
  [[ -z "$web_want" || "$(image_id "$web_img")" == "$web_want" ]] || die "Tag $web_img sudah menunjuk image lain dari yang dicatat rilis."

  prev="${DEMO_RELEASE:-}"
  info "Demo: ${prev:-(kosong)} → $(bold "$rel")$([[ $reset == 1 ]] && echo '  + data baru dari produksi')"
  {
    echo "# Rilis yang sedang berjalan di demo (ditulis deploy/demo.sh $(now_iso))"
    echo "DEMO_RELEASE=$rel"
    echo "DEMO_APP_IMAGE=$app_img"
    echo "DEMO_WEB_IMAGE=$web_img"
  } > "$DEMO_STATE"
  load_env
  install_nginx_conf

  if [[ $reset == 1 ]]; then
    do_reset
  else
    DEMO_UP_SINCE=$(utc_stamp)
    dc up -d >/dev/null 2>&1 || dc up -d
  fi

  local health healthy=true
  if health=$(check_app_health "$DEMO_APP" 240 "$DEMO_UP_SINCE"); then
    ok "demo sehat ($health)"
  else
    healthy=false
    warn "demo TIDAK sehat: $health"
  fi
  local snap; snap=$(jq -r '.data.snapshot_at // empty' "$DEMO_STATUS" 2>/dev/null || true)
  jq -n --arg rel "$rel" --arg at "$(now_iso)" --arg prev "$prev" --arg health "$health" \
        --arg snap "$snap" --argjson healthy "$healthy" --argjson reset "$([[ $reset == 1 ]] && echo true || echo false)" \
    '{release: $rel, deployed_at: $at, previous_release: $prev, data_reset: $reset,
      data_snapshot_at: $snap, healthy: $healthy, health: $health}' > "$d/demo.json"
  status_set '.release' "$(jq -c . "$d/demo.json")"
  [[ $healthy == true ]] || die "Lihat log: deploy/demo.sh logs app"
  echo
  if [[ "$(jq -r '.source.mode' "$d/build.json")" == adopted-prod ]]; then
    echo "   Demo kini menjalankan versi yang sama dengan produksi."
  else
    echo "   Uji di https://$DEMO_HOST — bila sudah oke: deploy/release.sh promote $rel"
  fi
}

cmd_status() {
  load_env
  echo "$(bold 'Rilis demo')   : ${DEMO_RELEASE:-(belum ada)}"
  echo "  app           : ${DEMO_APP_IMAGE:-?}"
  echo "  web           : ${DEMO_WEB_IMAGE:-?}"
  if [[ -f "$DEMO_STATUS" ]]; then
    echo "  data produksi : $(jq -r '.data.snapshot_at // "?"' "$DEMO_STATUS") (salinan terakhir)"
    echo "  deploy        : $(jq -r '.release.deployed_at // "?"' "$DEMO_STATUS")"
  fi
  echo "$(bold 'Container')"
  dc ps --format 'table {{.Service}}\t{{.Status}}' 2>/dev/null | sed 's/^/  /'
  if container_running "$DEMO_APP" && docker exec "$DEMO_APP" wget -qO- -T 3 http://127.0.0.1:4000/api/v1/ping >/dev/null 2>&1; then
    ok "API demo menjawab"
  else
    warn "API demo tidak menjawab"
  fi
  echo "$(bold 'Jalur domain')"
  if prod_web_on_edge; then ok "nginx produksi tersambung ke $EDGE_NETWORK"; else warn "nginx produksi BELUM tersambung ke $EDGE_NETWORK (deploy/demo.sh route-on)"; fi
  if prod_web_has_route; then ok "nginx produksi punya blok $DEMO_HOST"; else warn "nginx produksi BELUM punya blok $DEMO_HOST (deploy/demo.sh route-on)"; fi
  if getent hosts "$DEMO_HOST" >/dev/null 2>&1; then ok "DNS $DEMO_HOST sudah ada"; else warn "DNS $DEMO_HOST belum ada (Cloudflare: CNAME demo → $PROD_HOST, Proxied)"; fi
}

cmd_route_on() {
  take_lock
  require_tools
  container_running "$PROD_WEB" || die "nginx produksi ($PROD_WEB) tidak berjalan."
  ensure_edge_network
  mkdir -p "$DEMO_DIR/route-backup"

  if prod_web_on_edge; then
    ok "nginx produksi sudah tersambung ke $EDGE_NETWORK"
  else
    docker network connect "$EDGE_NETWORK" "$PROD_WEB"
    ok "nginx produksi disambungkan ke $EDGE_NETWORK (tanpa restart)"
  fi

  if prod_web_has_route; then
    ok "nginx produksi sudah punya blok $DEMO_HOST"
  else
    local block backup tmp
    block=$(sed -n "/^$EDGE_MARK_BEGIN/,/^$EDGE_MARK_END/p" "$REPO_DIR/nginx/default.http-only.conf")
    [[ -n "$block" ]] || die "Blok $DEMO_HOST tidak ditemukan di nginx/default.http-only.conf"
    backup="$DEMO_DIR/route-backup/default.conf.$(now_ts)"
    docker cp "$PROD_WEB:/etc/nginx/conf.d/default.conf" "$backup"
    tmp=$(mktemp "$DEMO_DIR/route-backup/.new.XXXXXX")
    { cat "$backup"; printf '\n%s\n' "$block"; } > "$tmp"
    chmod 644 "$tmp"
    docker cp "$tmp" "$PROD_WEB:/etc/nginx/conf.d/default.conf"
    rm -f "$tmp"
    if ! docker exec "$PROD_WEB" nginx -t >/dev/null 2>&1; then
      docker cp "$backup" "$PROD_WEB:/etc/nginx/conf.d/default.conf"
      docker exec "$PROD_WEB" nginx -t 2>&1 | tail -3 >&2
      die "Konfigurasi baru ditolak nginx -t; konfigurasi lama dikembalikan, produksi tidak berubah."
    fi
    docker exec "$PROD_WEB" nginx -s reload >/dev/null 2>&1
    ok "blok $DEMO_HOST dipasang & nginx produksi di-reload (tanpa downtime); cadangan: $backup"
    echo "     Blok yang sama sudah ada di nginx/default.http-only.conf, jadi ikut terpasang permanen"
    echo "     di image nginx rilis berikutnya."
  fi
  sleep 1
  cmd_route_check
}

cmd_route_check() {
  local fail=0 code hdr body
  hdr=$(curl -s -o /dev/null -D - -m 10 -H "Host: $PROD_HOST" http://127.0.0.1/ || true)
  code=$(awk 'NR==1{print $2}' <<<"$hdr")
  if [[ "$code" == 200 ]] && ! grep -qi '^x-nbp-environment' <<<"$hdr"; then
    ok "$PROD_HOST → produksi (HTTP $code)"
  else
    warn "$PROD_HOST → HTTP ${code:-?} $(grep -i '^x-nbp-environment' <<<"$hdr" | tr -d '\r')"; fail=1
  fi
  hdr=$(curl -s -o /dev/null -D - -m 10 -H "Host: $DEMO_HOST" http://127.0.0.1/ || true)
  code=$(awk 'NR==1{print $2}' <<<"$hdr")
  if [[ "$code" == 200 ]] && grep -qi '^x-nbp-environment: *demo' <<<"$hdr"; then
    ok "$DEMO_HOST → web demo (HTTP $code)"
  else
    warn "$DEMO_HOST → HTTP ${code:-?}, bukan web demo"; fail=1
  fi
  body=$(curl -s -m 10 -H "Host: $DEMO_HOST" http://127.0.0.1/api/v1/ping || true)
  if grep -q '"success":true' <<<"$body"; then ok "$DEMO_HOST/api → API demo"; else warn "$DEMO_HOST/api/v1/ping: ${body:0:120}"; fail=1; fi
  if getent hosts "$DEMO_HOST" >/dev/null 2>&1; then
    code=$(curl -s -o /dev/null -w '%{http_code}' -m 15 "https://$DEMO_HOST/api/v1/ping" || true)
    hdr=$(curl -s -o /dev/null -D - -m 15 "https://$DEMO_HOST/" || true)
    if [[ "$code" == 200 ]] && grep -qi '^x-nbp-environment: *demo' <<<"$hdr"; then
      ok "https://$DEMO_HOST (lewat Cloudflare) → demo"
    else
      warn "https://$DEMO_HOST → HTTP $code, penanda demo $(grep -qi '^x-nbp-environment' <<<"$hdr" && echo ada || echo TIDAK ada)"; fail=1
    fi
  else
    warn "DNS $DEMO_HOST belum ada — Cloudflare: CNAME demo → $PROD_HOST, Proxied"
  fi
  return $fail
}

cmd_outlet_keys() {
  load_env
  docker exec "$DEMO_DB" psql -U postgres -d "$DEMO_DB_NAME" -P pager=off \
    -c "SELECT code AS kode, name AS outlet, trim(id) AS outlet_id, api_key AS kunci_api_demo
          FROM outlets WHERE is_active ORDER BY code"
  echo "Tablet latihan: Pengaturan → Cloud → mode Development, URL https://$DEMO_HOST,"
  echo "lalu isi Outlet ID & kunci di atas. Kunci ini tidak berlaku di produksi."
}

cmd_destroy() {
  take_lock
  load_env
  warn "Ini menghapus container DAN volume demo (basis data, unggahan, dokumen demo)."
  warn "Produksi tidak tersentuh. Rahasia di $DEMO_ENV dibiarkan."
  confirm_typed "Ketik 'hapus demo' untuk melanjutkan:" "hapus demo"
  dc down -v --remove-orphans
  rm -f "$DEMO_STATE"
  ok "demo dihapus. Pasang ulang: deploy/demo.sh init"
}

main() {
  local cmd="${1:-help}"; shift || true
  case "$cmd" in
    init)        cmd_init "$@" ;;
    reset)       cmd_reset ;;
    deploy)      cmd_deploy "$@" ;;
    status)      cmd_status ;;
    logs)        load_env; dc logs --tail=200 -f "${1:-app}" ;;
    start)       take_lock; load_env; install_nginx_conf; dc up -d; cmd_status ;;
    stop)        take_lock; load_env; dc stop; ok "demo dimatikan; demo.nbp.co.id menampilkan halaman 'demo tidak aktif'" ;;
    route-on)    cmd_route_on ;;
    route-check) cmd_route_check ;;
    outlet-keys) cmd_outlet_keys ;;
    training-users)
      take_lock; load_env
      [[ -f "$TRAINING_CSV" ]] || die "Belum ada $TRAINING_CSV — salin dari deploy/demo/training-users.example.csv lalu isi."
      apply_training_users ;;
    psql)        load_env; docker exec -it "$DEMO_DB" psql -U postgres -d "$DEMO_DB_NAME" ;;
    destroy)     cmd_destroy ;;
    help|-h|--help) usage ;;
    *) usage; die "Perintah tidak dikenal: $cmd" ;;
  esac
}

main "$@"
