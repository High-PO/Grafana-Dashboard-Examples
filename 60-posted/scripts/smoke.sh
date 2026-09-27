#!/usr/bin/env bash
# Terraform 없이 Grafana HTTP API 로 대시보드 JSON 을 올려보고, 다시 읽어서 확인하는 스모크 테스트.
#
#   GRAFANA_URL=http://localhost:3000 GRAFANA_AUTH=admin:admin ./scripts/smoke.sh
#
# 확인하는 것:
#   1. Grafana 가 JSON 을 받아들이는지 (POST /api/dashboards/db → 200)
#   2. 저장 후 다시 읽었을 때 uid / title / 패널 수가 그대로인지
#
# 테스트가 끝나면 올린 대시보드와 폴더를 삭제합니다. (뒤이어 Terraform 으로 같은 UID 를 만들 수 있도록)
# 주의: 같은 UID 의 대시보드를 덮어쓴 뒤 삭제하므로 운영(홈랩) Grafana 에는 절대 돌리지 마세요. 로컬/CI 전용입니다.
set -euo pipefail

GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"
GRAFANA_AUTH="${GRAFANA_AUTH:-admin:admin}"
DIST_DIR="${DIST_DIR:-$(cd "$(dirname "$0")/.." && pwd)/dist}"
FOLDER_UID="smoke-test"

api() {
  curl -sS -u "$GRAFANA_AUTH" -H "Content-Type: application/json" "$@"
}

cleanup() {
  for file in "$DIST_DIR"/*.json; do
    api -o /dev/null -X DELETE "$GRAFANA_URL/api/dashboards/uid/$(jq -r '.uid' "$file")" || true
  done
  api -o /dev/null -X DELETE "$GRAFANA_URL/api/folders/$FOLDER_UID" || true
}
trap cleanup EXIT

# 스모크 테스트 전용 폴더 (이미 있으면 409 가 나오므로 무시)
api -o /dev/null -X POST "$GRAFANA_URL/api/folders" \
  -d "{\"uid\":\"$FOLDER_UID\",\"title\":\"Smoke Test\"}" || true

failed=0
for file in "$DIST_DIR"/*.json; do
  uid=$(jq -r '.uid' "$file")
  title=$(jq -r '.title' "$file")
  want_panels=$(jq '[.panels[], (.panels[].panels // [])[]] | length' "$file")

  payload=$(jq -c --arg folder "$FOLDER_UID" \
    '{dashboard: ., folderUid: $folder, overwrite: true, message: "smoke test"}' "$file")

  status=$(api -o /tmp/smoke-post.json -w '%{http_code}' -X POST "$GRAFANA_URL/api/dashboards/db" -d "$payload")
  if [[ "$status" != "200" ]]; then
    echo "FAIL  $uid: POST 응답 $status"
    cat /tmp/smoke-post.json; echo
    failed=1
    continue
  fi

  saved=$(api "$GRAFANA_URL/api/dashboards/uid/$uid")
  got_title=$(jq -r '.dashboard.title' <<<"$saved")
  got_panels=$(jq '[.dashboard.panels[], (.dashboard.panels[].panels // [])[]] | length' <<<"$saved")

  if [[ "$got_title" != "$title" || "$got_panels" != "$want_panels" ]]; then
    echo "FAIL  $uid: title='$got_title' panels=$got_panels (기대값: '$title' / $want_panels)"
    failed=1
    continue
  fi

  echo "OK    $uid ($got_panels panels) → $GRAFANA_URL$(jq -r '.meta.url' <<<"$saved")"
done

exit "$failed"
