---
inclusion: always
---

# 62-posted: grafanalib vs Grafana Foundation SDK

이 폴더는 같은 Grafana 대시보드("Node Overview", SPEC.md)를 세 가지로 구현하고,
서로 마이그레이션하는 방법을 보여주는 블로그 예제입니다.

## 구성
- `grafanalib/node_overview.dashboard.py` — grafanalib 0.7.1 (Python)
- `foundation_sdk_python/node_overview.py` — Grafana Foundation SDK 0.0.20 (Python)
- `foundation_sdk_go/main.go` — Grafana Foundation SDK v0.0.20 (Go)
- `tools/compare.py` — 대시보드 JSON 의미 비교 / `--exact` 완전 비교
- `tools/normalize_grafanalib.py` — grafanalib JSON 을 SDK 변환기에 넣을 수 있게 정리
- `foundation_sdk_go/cmd/convert` — JSON → Foundation SDK Go 코드 초안
- `tasks.py` — 모든 작업의 진입점 (build / compare / check / convert / up / smoke / down)
- `dist/` — 생성된 JSON (커밋 대상, 직접 수정 금지)

## 규칙
- 대시보드를 바꾸면 **세 구현을 모두** 같이 바꾸고 `python tasks.py check` 가 통과해야 합니다.
  - grafanalib ↔ SDK: 의미 비교 통과
  - SDK Python ↔ SDK Go: JSON 이 완전히 같아야 함 (`--exact`)
- grafana-foundation-sdk 는 반드시 `==0.0.20` 으로 고정합니다 (PyPI epoch 버전 문제).
- 대상 Grafana 는 12.0.1, schemaVersion 은 41 로 맞춥니다.
- grafanalib 이 지원하지 않는 옵션은 `extraJson` 으로 넣습니다.
- Foundation SDK 의 enum 옵션은 명시적으로 지정합니다 (Python/Go 기본값이 다름).
- 사용자는 Windows PowerShell 을 씁니다. 셸 리다이렉트(`>`) 대신 `tasks.py` 를 사용하세요.
