"""grafanalib 이 만든 JSON 을 "요즘 Grafana JSON" 모양으로 정리합니다.

    python tools/normalize_grafanalib.py dist/grafanalib.json dist/grafanalib.normalized.json

왜 필요한가?
    grafanalib 은 오래된 Graph 패널 시절의 필드와 기본값을 전부 출력합니다. 특히
      - 첫 번째 임계값을 "value": "null" (문자열!) 로 출력해서
        Foundation SDK(Go)의 JSON → 코드 변환기가 아예 읽지 못합니다.
          json: cannot unmarshal string into ... thresholds.steps.value of type float64
      - targets 에 query / target / metric / step / intervalFactor 같은 레거시 필드가 붙고
      - null / 빈 문자열 / 빈 목록 필드가 수십 개씩 들어갑니다.
    이 스크립트는 화면/동작에 영향이 없는 것만 지워서, SDK 변환기에 넣을 수 있는 JSON 으로 만듭니다.

Grafana 에 import 한 뒤 UI 에서 "Export → JSON" 해도 비슷하게 정리되지만,
이 스크립트는 Grafana 없이 로컬에서 바로 돌릴 수 있습니다.
"""

import json
import sys

# 최상위에서 지울 레거시 필드 (Grafana 5 이전 rows 레이아웃 등)
LEGACY_DASHBOARD_KEYS = {"rows", "style", "sharedCrosshair", "hideControls", "__inputs", "gnetId", "id", "version"}

# 패널에서 지울 레거시 필드
LEGACY_PANEL_KEYS = {"span", "height", "minSpan", "error", "editable"}

# 쿼리(target)에서 지울 레거시 필드. expr 와 같은 값이 query 에 중복으로 들어갑니다.
LEGACY_TARGET_KEYS = {"query", "target", "metric", "step", "intervalFactor"}

# 임계값 step 에서 지울 Graph 패널 시절 필드
LEGACY_THRESHOLD_KEYS = {"op", "yaxis", "line", "index"}


def normalize(d: dict) -> dict:
    d = {k: v for k, v in d.items() if k not in LEGACY_DASHBOARD_KEYS}

    for v in (d.get("templating") or {}).get("list") or []:
        # grafanalib 은 current 를 {"text": null, "value": null} 로 채워서 넣습니다.
        if not (v.get("current") or {}).get("value"):
            v.pop("current", None)

    d["panels"] = [normalize_panel(p) for p in d.get("panels") or []]
    return drop_empty(d)


def normalize_panel(p: dict) -> dict:
    p = {k: v for k, v in p.items() if k not in LEGACY_PANEL_KEYS}

    p["targets"] = [
        {k: v for k, v in t.items() if k not in LEGACY_TARGET_KEYS} for t in p.get("targets") or []
    ]

    thresholds = ((p.get("fieldConfig") or {}).get("defaults") or {}).get("thresholds")
    if thresholds:
        steps = []
        for s in thresholds.get("steps") or []:
            s = {k: v for k, v in s.items() if k not in LEGACY_THRESHOLD_KEYS}
            if s.get("value") == "null":
                s["value"] = None  # 문자열 "null" → JSON null (-∞)
            steps.append(s)
        thresholds["steps"] = steps

    if p.get("type") == "row":
        p["panels"] = [normalize_panel(c) for c in p.get("panels") or []]

    return p


def drop_empty(v, keep_null=False):
    """null / 빈 문자열 / 빈 목록·객체 필드를 지웁니다. 단, 임계값의 "value": null(-∞)은 유지합니다."""
    if isinstance(v, dict):
        out = {}
        for k, val in v.items():
            if k == "steps" and isinstance(val, list):
                out[k] = [drop_empty(s, keep_null=True) for s in val]
                continue
            c = drop_empty(val)
            if c is None and keep_null and k == "value":
                out[k] = None
                continue
            if c in (None, "", [], {}):
                continue
            out[k] = c
        return out
    if isinstance(v, list):
        return [drop_empty(x) for x in v]
    return v


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: normalize_grafanalib.py <input.json> <output.json>", file=sys.stderr)
        return 2

    with open(sys.argv[1], encoding="utf-8") as f:
        dash = json.load(f)

    with open(sys.argv[2], "w", encoding="utf-8") as f:
        json.dump(normalize(dash), f, indent=2, ensure_ascii=False, sort_keys=True)
        f.write("\n")

    print(f"normalized: {sys.argv[1]} → {sys.argv[2]}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
