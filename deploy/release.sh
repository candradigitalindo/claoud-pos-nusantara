#!/usr/bin/env bash
# Skema rilis: build SEKALI → uji di demo → naikkan image yang SAMA PERSIS ke produksi.
# Panduan lengkap: docs/lingkungan-demo.md
#
#   deploy/release.sh build <nama> [--ref <commit>]   bangun image kandidat (bawaan: working tree)
#   deploy/release.sh list                            daftar rilis & posisinya (demo / produksi)
#   deploy/release.sh show <rilis>                    rincian satu rilis
#   deploy/release.sh promote <rilis> [--yes] [--allow-dirty] [--skip-demo]
#                                                     naikkan ke produksi (cadangan DB + rollback otomatis)
#   deploy/release.sh rollback <rilis> [--yes]        kembalikan produksi ke image sebelum rilis itu
#   deploy/release.sh adopt-prod <nama>               catat image produksi yang sedang jalan sebagai rilis

source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

usage() { sed -n '2,/^$/{s/^# \{0,1\}//;p}' "$0"; }

# Compose produksi persis seperti biasa (proyek cloud-pos, .env di akar repo),
# ditambah berkas penimpa image milik rilis.
dprod() { docker compose -p "$PROD_PROJECT" --project-directory "$REPO_DIR" -f "$PROD_COMPOSE" "$@"; }

valid_label() { [[ "$1" =~ ^[a-z0-9][a-z0-9-]{1,40}$ ]] || die "Nama rilis '$1' tidak sah: pakai huruf kecil, angka, dan minus (mis. rekap-darurat)."; }

write_compose_override() { # berkas app-image web-image
  jq -n --arg app "$2" --arg web "$3" \
    '{services: {app: {image: $app}, nginx: {image: $web}}}' > "$1"
}

# ── build ────────────────────────────────────────────────────────────────────
cmd_build() {
  local label="" ref="" a
  while (($#)); do
    case "$1" in
      --ref) ref="${2:-}"; [[ -n "$ref" ]] || die "--ref butuh nama commit/branch/tag"; shift 2 ;;
      -*) die "Opsi tidak dikenal: $1" ;;
      *) label="$1"; shift ;;
    esac
  done
  [[ -n "$label" ]] || die "Pakai: deploy/release.sh build <nama> [--ref <commit>]"
  valid_label "$label"
  take_lock
  require_tools

  local ts id tag d src mode commit branch dirty=false changed=0
  ts=$(now_ts); id="$ts-$label"; tag="$label-$ts"
  d="$RELEASES_DIR/$id"
  mkdir -p "$d"
  src=$(mktemp -d "$RELEASES_DIR/.src-XXXXXX")
  # shellcheck disable=SC2064
  trap "rm -rf '$src'" EXIT
  branch=$(git -C "$REPO_DIR" rev-parse --abbrev-ref HEAD)

  if [[ -n "$ref" ]]; then
    mode=git-ref
    commit=$(git -C "$REPO_DIR" rev-parse --verify --quiet "$ref^{commit}") || die "Commit '$ref' tidak ditemukan."
    git -C "$REPO_DIR" archive "$commit" | tar -x -C "$src"
  else
    mode=worktree
    commit=$(git -C "$REPO_DIR" rev-parse HEAD)
    git -C "$REPO_DIR" status --porcelain=v1 > "$d/git-status.txt"
    if [[ -s "$d/git-status.txt" ]]; then
      dirty=true
      changed=$(wc -l < "$d/git-status.txt")
      # Jejak sumber yang belum di-commit: diff berkas terlacak + arsip berkas
      # baru (≤ 5 MB; yang lebih besar hanya dicatat namanya).
      git -C "$REPO_DIR" diff --binary HEAD > "$d/changes.patch"
      (
        cd "$REPO_DIR"
        git ls-files -o --exclude-standard -z | while IFS= read -r -d '' f; do
          if (( $(stat -c %s "$f") <= 5242880 )); then printf '%s\0' "$f"
          else printf '%s\t%s\n' "$(stat -c %s "$f")" "$f" >> "$d/untracked-large.txt"; fi
        done > "$d/.untracked.list"
        if [[ -s "$d/.untracked.list" ]]; then tar --null -T "$d/.untracked.list" -czf "$d/untracked.tar.gz"; fi
        rm -f "$d/.untracked.list"
      )
    fi
    # Konteks build = berkas terlacak + berkas baru yang tidak di-.gitignore,
    # persis seperti yang akan di-commit; sampah lokal (laporan/, uploads/,
    # node_modules/) tidak pernah ikut.
    (
      cd "$REPO_DIR"
      git ls-files -co --exclude-standard -z | sort -zu | while IFS= read -r -d '' f; do
        [[ -e "$f" || -L "$f" ]] && printf '%s\0' "$f"
      done | tar --null -T - -cf -
    ) | tar -x -C "$src"
  fi
  echo "$commit" > "$d/git-head.txt"

  info "Membangun rilis $(bold "$id")"
  echo "     sumber : $mode ${commit:0:7} ($branch)$([[ $dirty == true ]] && echo " + $changed berkas belum di-commit")"
  echo "     image  : $APP_REPO:$tag, $WEB_REPO:$tag   (log: $d/build.log)"
  date --iso-8601=seconds > "$d/build-started.txt"
  if ! { docker build --target api -t "$APP_REPO:$tag" "$src" \
         && docker build --target frontend -t "$WEB_REPO:$tag" "$src"; } > "$d/build.log" 2>&1; then
    tail -25 "$d/build.log" >&2
    die "Build gagal (log lengkap: $d/build.log)"
  fi

  jq -n --arg id "$id" --arg label "$label" --arg at "$(now_iso)" \
        --arg mode "$mode" --arg ref "$ref" --arg commit "$commit" --arg branch "$branch" \
        --argjson dirty "$dirty" --argjson changed "$changed" \
        --arg app "$APP_REPO:$tag" --arg app_id "$(image_id "$APP_REPO:$tag")" \
        --arg web "$WEB_REPO:$tag" --arg web_id "$(image_id "$WEB_REPO:$tag")" \
    '{release: $id, label: $label, created_at: $at,
      source: {mode: $mode, ref: $ref, commit: $commit, branch: $branch, dirty: $dirty, files_changed: $changed},
      images: {app: {tag: $app, id: $app_id}, web: {tag: $web, id: $web_id}}}' > "$d/build.json"
  write_compose_override "$d/compose.release.json" "$APP_REPO:$tag" "$WEB_REPO:$tag"
  ok "rilis $id siap"
  echo
  echo "   Langkah berikut: deploy/demo.sh deploy $id --reset"
  if [[ $dirty == true ]]; then
    warn "Rilis ini memuat perubahan yang belum di-commit. Promosi ke produksi akan menolak tanpa --allow-dirty;"
    warn "lebih baik commit dulu lalu build ulang supaya produksi selalu bisa dilacak ke satu commit."
  fi
}

# ── adopt-prod: catat image produksi yang sedang jalan sebagai rilis ─────────
cmd_adopt_prod() {
  local label="${1:-prod-baseline}"
  valid_label "$label"
  take_lock
  require_tools
  container_running "$PROD_APP" || die "API produksi ($PROD_APP) tidak berjalan."
  container_running "$PROD_WEB" || die "nginx produksi ($PROD_WEB) tidak berjalan."
  local app_id web_id ts id tag d started commit
  app_id=$(container_image_id "$PROD_APP"); web_id=$(container_image_id "$PROD_WEB")
  ts=$(now_ts); id="$ts-$label"; tag="$label-$ts"
  d="$RELEASES_DIR/$id"
  mkdir -p "$d"
  docker tag "$app_id" "$APP_REPO:$tag"
  docker tag "$web_id" "$WEB_REPO:$tag"
  started=$(docker inspect -f '{{.State.StartedAt}}' "$PROD_APP")
  commit=$(git -C "$REPO_DIR" rev-parse HEAD)
  git -C "$REPO_DIR" status --porcelain=v1 > "$d/git-status.txt"
  echo "$commit" > "$d/git-head.txt"
  jq -n --arg id "$id" --arg label "$label" --arg at "$(now_iso)" --arg commit "$commit" \
        --arg branch "$(git -C "$REPO_DIR" rev-parse --abbrev-ref HEAD)" --arg started "$started" \
        --arg app "$APP_REPO:$tag" --arg app_id "$app_id" --arg web "$WEB_REPO:$tag" --arg web_id "$web_id" \
    '{release: $id, label: $label, created_at: $at,
      source: {mode: "adopted-prod", ref: "", commit: $commit, branch: $branch, dirty: false, files_changed: 0,
               note: ("Image yang sedang berjalan di produksi (container dibuat " + $started + "). Sumber persisnya tidak tercatat oleh alur rilis; commit = HEAD repo saat dicatat.")},
      images: {app: {tag: $app, id: $app_id}, web: {tag: $web, id: $web_id}}}' > "$d/build.json"
  write_compose_override "$d/compose.release.json" "$APP_REPO:$tag" "$WEB_REPO:$tag"
  jq -n --arg at "$started" '{status: "deployed", deployed_at: $at, note: "sudah berjalan di produksi saat dicatat (adopt-prod)"}' > "$d/deployment.json"
  info "Image produksi dicatat sebagai rilis $id ($APP_REPO:$tag, $WEB_REPO:$tag)" >&2
  echo "$id"
}

# ── list / show ──────────────────────────────────────────────────────────────
cmd_list() {
  local prod_app demo_app d id
  prod_app=$(container_image_id "$PROD_APP"); demo_app=$(container_image_id "$DEMO_APP")
  printf '%-44s %-27s %-18s %-12s %s\n' RILIS SUMBER DEMO PRODUKSI ""
  for d in "$RELEASES_DIR"/*/; do
    [[ -d "$d" ]] || continue
    id=$(basename "$d")
    local src="(rilis lama)" demo="-" prod="-" mark="" app_id=""
    if [[ -f "$d/build.json" ]] && is_release_record "$d/build.json"; then
      src=$(jq -r '.source | (.mode | sub("worktree";"tree") | sub("adopted-prod";"adopsi")) + " " + .commit[0:7] + (if .dirty then "+belum-commit" else "" end)' "$d/build.json")
      app_id=$(jq -r '.images.app.id // empty' "$d/build.json")
      [[ -f "$d/demo.json" ]] && demo=$(jq -r '(if .healthy then "lulus " else "GAGAL " end) + (.deployed_at[5:16] | sub("T";" "))' "$d/demo.json")
    elif [[ -f "$d/deployment.json" ]]; then
      app_id=$(jq -r '.images.app.deployed_id // empty' "$d/deployment.json" 2>/dev/null || true)
    else
      continue
    fi
    if [[ -f "$d/deployment.json" ]]; then
      prod=$(jq -r '(.status // "deployed") as $s | (if $s == "deployed" then "naik " elif $s == "rolled_back" then "rollback " else $s + " " end) + ((.deployed_at // "")[5:10])' "$d/deployment.json" 2>/dev/null || echo "?")
    fi
    [[ -n "$app_id" && "$app_id" == "$prod_app" ]] && mark+=" ◀ PRODUKSI"
    [[ -n "$app_id" && -n "$demo_app" && "$app_id" == "$demo_app" ]] && mark+=" ◀ DEMO"
    printf '%-44s %-27s %-18s %-12s %s\n' "$id" "$src" "$demo" "$prod" "$mark"
  done
}

cmd_show() {
  local d; d=$(release_dir "${1:-}")
  jq . "$d/build.json"
  [[ -f "$d/demo.json" ]] && { echo "── demo"; jq . "$d/demo.json"; }
  [[ -f "$d/deployment.json" ]] && { echo "── produksi"; jq '{status, deployed_at, health, database: .database.backup, rollback: .commands.rollback}' "$d/deployment.json"; }
  return 0
}

# ── promote ──────────────────────────────────────────────────────────────────
cmd_promote() {
  local rel="" yes=0 allow_dirty=0 skip_demo=0 a
  for a in "$@"; do
    case "$a" in
      --yes) yes=1 ;;
      --allow-dirty) allow_dirty=1 ;;
      --skip-demo) skip_demo=1 ;;
      -*) die "Opsi tidak dikenal: $a" ;;
      *) rel="$a" ;;
    esac
  done
  [[ -n "$rel" ]] || die "Pakai: deploy/release.sh promote <rilis>   (daftar: deploy/release.sh list)"
  take_lock
  require_tools

  local d app_img web_img app_want web_want mode dirty tag
  d=$(release_dir "$rel")
  app_img=$(jq -r '.images.app.tag' "$d/build.json"); web_img=$(jq -r '.images.web.tag' "$d/build.json")
  app_want=$(jq -r '.images.app.id' "$d/build.json"); web_want=$(jq -r '.images.web.id' "$d/build.json")
  mode=$(jq -r '.source.mode' "$d/build.json"); dirty=$(jq -r '.source.dirty' "$d/build.json")
  tag="${app_img#*:}"
  [[ "$(image_id "$app_img")" == "$app_want" ]] || die "Image $app_img hilang atau sudah menunjuk image lain."
  [[ "$(image_id "$web_img")" == "$web_want" ]] || die "Image $web_img hilang atau sudah menunjuk image lain."

  local cur_app cur_web
  cur_app=$(container_image_id "$PROD_APP"); cur_web=$(container_image_id "$PROD_WEB")
  [[ -n "$cur_app" && -n "$cur_web" ]] || die "Container produksi ($PROD_APP / $PROD_WEB) tidak ditemukan."
  if [[ "$cur_app" == "$app_want" && "$cur_web" == "$web_want" ]]; then
    ok "Rilis $rel sudah berjalan di produksi."
    return 0
  fi

  # ── gerbang ──
  if [[ ! -f "$d/demo.json" || "$(jq -r '.healthy' "$d/demo.json")" != true ]]; then
    (( skip_demo )) || die "Rilis ini belum lulus di demo. Jalankan dulu: deploy/demo.sh deploy $rel --reset   (darurat: --skip-demo)"
    warn "MELEWATI uji demo (--skip-demo)"
  fi
  if [[ "$dirty" == true ]]; then
    (( allow_dirty )) || die "Rilis ini dibangun dari perubahan yang belum di-commit. Commit, build ulang, uji di demo — atau --allow-dirty bila memang darurat."
    warn "rilis memuat perubahan yang belum di-commit (--allow-dirty); jejaknya di $d/changes.patch"
  fi
  [[ "$mode" == adopted-prod ]] && warn "rilis ini hasil adopt-prod (dicatat dari produksi, bukan dibangun alur rilis)"

  echo
  echo "$(bold 'Naik ke PRODUKSI'): $rel"
  echo "  sumber        : $(jq -r '.source | .mode + " " + .commit[0:7] + " (" + .branch + ")" + (if .dirty then " + belum di-commit" else "" end)' "$d/build.json")"
  if [[ -f "$d/demo.json" ]]; then
    echo "  uji demo      : $(jq -r '(if .healthy then "lulus" else "GAGAL" end) + " " + .deployed_at + (if .data_reset then " (data produksi segar " + .data_snapshot_at + ")" else " (data demo lama: " + (.data_snapshot_at // "?") + ")" end)' "$d/demo.json")"
    [[ "$(jq -r '.data_reset' "$d/demo.json")" == true ]] || warn "uji demo tanpa --reset: migrasi belum digladi di salinan produksi terbaru"
  fi
  echo "  image sekarang: $(docker inspect -f '{{.Config.Image}}' "$PROD_APP") / $(docker inspect -f '{{.Config.Image}}' "$PROD_WEB")"
  echo "  image baru    : $app_img / $web_img"
  echo "  langkah       : cadangan DB → tandai image lama (rollback) → ganti app + nginx → cek kesehatan"
  echo "                  (gagal sehat = otomatis kembali ke image lama)"
  echo
  (( yes )) || confirm_typed "Ketik nama rilis untuk melanjutkan:" "$rel"

  ensure_edge_network
  local started; started=$(now_iso)
  date --iso-8601=seconds > "$d/deploy-started.txt"

  info "1/5 Cadangan basis data produksi"
  docker exec "$PROD_DB" pg_dump -Fc -U postgres -d "$PROD_DB_NAME" > "$d/db-before.dump.part"
  mv "$d/db-before.dump.part" "$d/db-before.dump"
  local bytes sha
  bytes=$(stat -c %s "$d/db-before.dump")
  (( bytes > 1000000 )) || die "Cadangan basis data terlalu kecil ($bytes byte) — dibatalkan, produksi belum disentuh."
  sha=$(sha256sum "$d/db-before.dump" | cut -d' ' -f1)
  echo "$sha" > "$d/db-before.sha256"
  ok "$(numfmt --to=iec "$bytes")  $d/db-before.dump"

  info "2/5 Menandai image produksi saat ini untuk rollback"
  docker tag "$cur_app" "$APP_REPO:before-$tag"
  docker tag "$cur_web" "$WEB_REPO:before-$tag"
  write_compose_override "$d/compose.rollback.json" "$APP_REPO:before-$tag" "$WEB_REPO:before-$tag"
  jq -n --arg app "$(docker inspect -f '{{.Config.Image}}' "$PROD_APP")" --arg app_id "$cur_app" \
        --arg web "$(docker inspect -f '{{.Config.Image}}' "$PROD_WEB")" --arg web_id "$cur_web" \
    '{app: {image: $app, id: $app_id}, nginx: {image: $web, id: $web_id}}' > "$d/images-before.json"
  ok "$APP_REPO:before-$tag, $WEB_REPO:before-$tag"

  info "3/5 Mengganti app + nginx produksi"
  local since; since=$(utc_stamp)
  # Kegagalan compose tidak menghentikan skrip di sini: cek kesehatan di
  # bawah yang memutuskan, dan bila gagal produksi dikembalikan otomatis.
  if ! dprod -f "$d/compose.release.json" up -d --no-build --no-deps app nginx 2>&1 | tee "$d/deploy.log" | sed 's/^/     /'; then
    warn "docker compose melaporkan galat (lihat $d/deploy.log)"
  fi

  info "4/5 Memeriksa kesehatan produksi"
  local health status=deployed
  if ! health=$(check_app_health "$PROD_APP" 180 "$since"); then
    warn "Produksi TIDAK sehat ($health) — kembali otomatis ke image sebelumnya"
    docker logs --since "$since" "$PROD_APP" > "$d/failed-app.log" 2>&1 || true
    since=$(utc_stamp)
    dprod -f "$d/compose.rollback.json" up -d --no-build --no-deps app nginx 2>&1 | tee -a "$d/deploy.log" | sed 's/^/     /' || true
    local rb_health; rb_health=$(check_app_health "$PROD_APP" 180 "$since" || true)
    jq -n --arg at "$started" --arg h "$health" --arg rb "$rb_health" --slurpfile b "$d/build.json" \
      '{release: $b[0].release, deployed_at: $at, status: "rolled_back_auto", health: $h, rollback_health: $rb,
        note: "Gagal cek kesehatan; image lama dinaikkan kembali otomatis. Log app gagal: failed-app.log"}' > "$d/deployment.json"
    die "Rilis dibatalkan; produksi kembali ke image lama ($rb_health). Migrasi aditif yang sempat jalan dibiarkan; cadangan DB: $d/db-before.dump"
  fi
  ok "API: $health"
  local idx code
  idx=$(curl -s -m 10 -H "Host: $PROD_HOST" http://127.0.0.1/ || true)
  code=$(curl -s -o /dev/null -w '%{http_code}' -m 10 -H "Host: $PROD_HOST" http://127.0.0.1/ || true)
  if [[ "$code" == 200 ]] && grep -q '/assets/index-' <<<"$idx"; then
    ok "UI: index.html 200 ($(grep -o '/assets/index-[^"]*\.js' <<<"$idx" | head -1))"
  else
    warn "UI: HTTP $code — periksa nginx produksi"
    status=deployed_check_ui
  fi

  info "5/5 Mencatat rilis"
  docker tag "$app_img" "$APP_REPO:latest"
  docker tag "$web_img" "$WEB_REPO:latest"
  jq -n --arg at "$started" --arg status "$status" --arg health "$health" \
        --arg app_before "$cur_app" --arg web_before "$cur_web" \
        --arg app_rb "$APP_REPO:before-$tag" --arg web_rb "$WEB_REPO:before-$tag" \
        --arg dump "$d/db-before.dump" --argjson bytes "$bytes" --arg sha "$sha" \
        --arg rollback "deploy/release.sh rollback $rel" \
        --slurpfile b "$d/build.json" \
        --argjson demo "$(if [[ -f "$d/demo.json" ]]; then cat "$d/demo.json"; else echo null; fi)" \
    '{release: $b[0].release, deployed_at: $at, status: $status, source: $b[0].source,
      images: {app: {before: {id: $app_before}, rollback_tag: $app_rb, release_tag: $b[0].images.app.tag, deployed_id: $b[0].images.app.id},
               nginx: {before: {id: $web_before}, rollback_tag: $web_rb, release_tag: $b[0].images.web.tag, deployed_id: $b[0].images.web.id},
               latest_retagged: true},
      database: {backup: {file: $dump, format: "pg_dump -Fc", bytes: $bytes, sha256: $sha}},
      demo: $demo, health: $health, commands: {rollback: $rollback}}' > "$d/deployment.json"
  ok "produksi menjalankan $rel"
  if container_running "$DEMO_WEB" && ! docker exec "$PROD_WEB" grep -q "server_name $DEMO_HOST;" /etc/nginx/conf.d/default.conf 2>/dev/null; then
    warn "nginx produksi yang baru tidak punya blok $DEMO_HOST — jalankan: deploy/demo.sh route-on"
  fi
  echo
  echo "   Bila ada masalah: deploy/release.sh rollback $rel"
}

# ── rollback ─────────────────────────────────────────────────────────────────
cmd_rollback() {
  local rel="" yes=0 a
  for a in "$@"; do
    case "$a" in
      --yes) yes=1 ;;
      -*) die "Opsi tidak dikenal: $a" ;;
      *) rel="$a" ;;
    esac
  done
  [[ -n "$rel" ]] || die "Pakai: deploy/release.sh rollback <rilis>"
  take_lock
  require_tools
  local d; d=$(release_dir "$rel")
  [[ -f "$d/compose.rollback.json" ]] || die "Rilis ini tidak pernah dinaikkan lewat release.sh (tidak ada compose.rollback.json)."
  local app_rb web_rb
  app_rb=$(jq -r '.services.app.image' "$d/compose.rollback.json")
  web_rb=$(jq -r '.services.nginx.image' "$d/compose.rollback.json")
  [[ -n "$(image_id "$app_rb")" && -n "$(image_id "$web_rb")" ]] || die "Image rollback ($app_rb / $web_rb) sudah tidak ada."
  echo "$(bold 'ROLLBACK produksi') ke image sebelum $rel: $app_rb / $web_rb"
  echo "  Basis data TIDAK dikembalikan (data sesudah rilis tetap aman)."
  (( yes )) || confirm_typed "Ketik 'rollback' untuk melanjutkan:" "rollback"
  local since health; since=$(utc_stamp)
  dprod -f "$d/compose.rollback.json" up -d --no-build --no-deps app nginx 2>&1 | tee -a "$d/deploy.log" | sed 's/^/     /' || true
  if health=$(check_app_health "$PROD_APP" 180 "$since"); then
    ok "produksi sehat ($health)"
  else
    warn "produksi belum sehat setelah rollback: $health — periksa: docker logs $PROD_APP"
  fi
  docker tag "$app_rb" "$APP_REPO:latest"
  docker tag "$web_rb" "$WEB_REPO:latest"
  jq -n --arg at "$(now_iso)" --arg health "$health" '{rolled_back_at: $at, health: $health}' > "$d/rollback.json"
  if [[ -f "$d/deployment.json" ]]; then
    local tmp; tmp=$(mktemp "$d/.dep.XXXXXX")
    jq '.status = "rolled_back"' "$d/deployment.json" > "$tmp" && mv "$tmp" "$d/deployment.json"
  fi
  echo "   Bila skema basis data harus ikut kembali (jarang; migrasi di sini aditif):"
  echo "   docker exec -i $PROD_DB pg_restore -U postgres -d $PROD_DB_NAME --clean --if-exists < $d/db-before.dump"
  echo "   (MENGHAPUS data yang masuk sesudah rilis — hanya bila benar-benar perlu)."
}

main() {
  local cmd="${1:-help}"; shift || true
  case "$cmd" in
    build)      cmd_build "$@" ;;
    adopt-prod) cmd_adopt_prod "$@" ;;
    list|ls)    cmd_list ;;
    show)       cmd_show "$@" ;;
    promote)    cmd_promote "$@" ;;
    rollback)   cmd_rollback "$@" ;;
    help|-h|--help) usage ;;
    *) usage; die "Perintah tidak dikenal: $cmd" ;;
  esac
}

main "$@"
