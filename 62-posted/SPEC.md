# Node Overview — 대시보드 명세

세 구현(grafanalib / Foundation SDK Python / Foundation SDK Go)이 모두 이 명세를 따릅니다.
`python tasks.py compare`로 세 결과가 같은지 자동으로 확인합니다.

## 대시보드

| 항목 | 값 |
| --- | --- |
| UID | `node-overview` |
| 제목 | Node Overview |
| 태그 | `node`, `homelab`, `sdk-compare` |
| 시간 범위 / 새로고침 | `now-3h` ~ `now` / 30s |
| 타임존 | browser |
| 툴팁 | 패널 간 crosshair 공유 (`graphTooltip: 1`) |
| schemaVersion | 41 (Grafana 12.0.x) |

## 변수

| 이름 | 종류 | 값 |
| --- | --- | --- |
| `datasource` | 데이터소스 | Prometheus 타입 |
| `loki` | 데이터소스 | Loki 타입 |
| `instance` | 쿼리 | `label_values(node_uname_info, instance)` · 다중 선택 · All 포함 · 시간 범위 변경 시 갱신 · 알파벳순 |

## 패널 (그리드 24칸)

| 행 | 패널 | 종류 | 크기 (w×h) | 내용 |
| --- | --- | --- | --- | --- |
| 요약 | Up | stat | 6×4 | `sum(up{job=~".*node.*"})` · 임계값 red → green(1) |
| | CPU 사용률 | stat | 6×4 | idle 제외 CPU 비율 · percentunit · 0~1 · green → orange(0.7) → red(0.9) |
| | Memory 사용률 | stat | 6×4 | 1 - Available/Total · 위와 같은 임계값 |
| | Root 디스크 사용률 | stat | 6×4 | `/` 파일시스템 · green → orange(0.8) → red(0.9) |
| 리소스 추이 | CPU 사용률 (인스턴스별) | timeseries | 12×8 | 인스턴스별 · 0~1 |
| | Memory 사용률 (인스턴스별) | timeseries | 12×8 | 인스턴스별 · 0~1 |
| | Network 송수신 | timeseries | 24×8 | rx(위) / tx(아래, override `negative-Y`) · Bps |
| 로그 | 에러 로그 | logs | 24×10 | Loki `{instance=~"$instance"} \|~ "(?i)(error\|fail\|panic)"` |

공통 옵션:
- **Stat:** lastNotNull · colorMode `value` · graphMode `area`
- **Timeseries:** line · 선 두께 1 · 채우기 10 · 점 표시 안 함 · 범례는 표 형태(하단, mean / max / lastNotNull) · 툴팁 multi(내림차순)
- **Logs:** 시간 표시 · 줄바꿈 · 로그 상세 · 최신순
