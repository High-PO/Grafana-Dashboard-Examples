"""두 대시보드 JSON 이 "같은 대시보드"인지 비교합니다.

    python tools/compare.py dist/grafanalib.json dist/fsdk-python.json
    python tools/compare.py --exact dist/fsdk-python.json dist/fsdk-go.json

기본 모드 (의미 비교)
    grafanalib 과 Foundation SDK 는 같은 대시보드라도 JSON 모양이 꽤 다릅니다.
      - grafanalib 은 레거시 필드(intervalFactor, step, op/yaxis 가 붙은 threshold ...)와
        기본값(null, false, "")을 전부 출력합니다.
      - Foundation SDK 는 지정한 값만 출력합니다.
    그래서 화면/동작에 영향을 주는 값(쿼리, 단위, 임계값, 배치, 변수, 주요 옵션)만 뽑아서 비교합니다.

--exact 모드
    키 순서만 무시하고 JSON 전체가 완전히 같은지 비교합니다.
    Foundation SDK Python ↔ Go 처럼 같은 SDK 의 다른 언어 구현을 비교할 때 씁니다.

차이가 있으면 목록을 출력하고 exit code 1 로 종료합니다.
"""

import argparse
import json
import sys

# 대시보드 최상위에서 비교할 필드
DASHBOARD_FIELDS = ["uid", "title", "description", "tags", "refresh", "time", "graphTooltip", "timezone", "schemaVersion"]

# 패널 options 중 비교할 경로
OPTION_PATHS = [
    ("reduceOptions", "calcs"),
    ("colorMode",),
    ("graphMode",),
    ("legend", "displayMode"),
    ("legend", "placement"),
    ("legend", "calcs"),
    ("tooltip", "mode"),
    ("tooltip", "sort"),
    ("showTime",),
    ("wrapLogMessage",),
    ("enableLogDetails",),
    ("sortOrder",),
]

# fieldConfig.defaults.custom 중 비교할 필드
CUSTOM_FIELDS = ["drawStyle", "lineWidth", "fillOpacity", "showPoints"]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--exact", action="store_true", help="키 순서만 무시하고 JSON 전체를 비교")
    parser.add_argument("left", help="기준 JSON (예: 마이그레이션 전)")
    parser.add_argument("right", help="비교 대상 JSON (예: 마이그레이션 후)")
    args = parser.parse_args()

    left, right = load(args.left), load(args.right)
    if not args.exact:
        left, right = project(left), project(right)

    diffs = diff("", left, right)
    if not diffs:
        mode = "JSON 전체가" if args.exact else "의미 있는 필드가 모두"
        print(f"OK: {mode} 같습니다.  ({args.left} == {args.right})")
        return 0

    print(f"{len(diffs)}개 차이 발견 (- {args.left} / + {args.right}):")
    for d in diffs:
        print(d)
    return 1


def load(path: str) -> dict:
    with open(path, encoding="utf-8") as f:
        return json.load(f)


# ---------------------------------------------------------------------------
# 비교용 구조로 변환
# ---------------------------------------------------------------------------


def project(d: dict) -> dict:
    out = pick(d, DASHBOARD_FIELDS)

    variables = {}
    for v in (d.get("templating") or {}).get("list") or []:
        p = pick(v, ["type", "label", "description", "regex", "multi", "includeAll", "allValue"])
        query = v.get("query")
        # 쿼리 변수: grafanalib 은 문자열, Foundation SDK 는 {query, refId} 객체
        p["query"] = query.get("query") if isinstance(query, dict) else query
        p["datasource"] = datasource(v.get("datasource"))
        if v.get("type") == "query":
            p.update(pick(v, ["refresh", "sort"]))
        variables[v["name"]] = p
    out["variables"] = variables

    panels = {}

    def walk(items):
        for p in items or []:
            key = f"[{p.get('type')}] {p.get('title')}"
            if p.get("type") == "row":
                panels[key] = pick(p, ["gridPos", "collapsed"])
                walk(p.get("panels"))
                continue
            panels[key] = project_panel(p)

    walk(d.get("panels"))
    out["panels"] = panels

    return clean(out)


def project_panel(p: dict) -> dict:
    out = pick(p, ["gridPos", "description", "transformations", "maxDataPoints", "interval"])
    out["datasource"] = datasource(p.get("datasource"))

    field_config = p.get("fieldConfig") or {}
    defaults = field_config.get("defaults") or {}
    out["defaults"] = pick(defaults, ["unit", "decimals", "min", "max", "color", "noValue"])
    out["defaults"]["thresholds"] = thresholds(defaults.get("thresholds"))
    out["custom"] = pick(defaults.get("custom") or {}, CUSTOM_FIELDS)
    out["overrides"] = field_config.get("overrides")

    options = p.get("options") or {}
    out["options"] = {".".join(path): dig(options, path) for path in OPTION_PATHS}

    targets = {}
    for t in p.get("targets") or []:
        tp = pick(t, ["expr", "legendFormat", "format", "instant"])
        if tp.get("format") == "time_series":  # Prometheus 기본값
            tp.pop("format")
        tp["datasource"] = datasource(t.get("datasource"))
        targets[t.get("refId")] = tp
    out["targets"] = targets

    return out


def datasource(ds):
    """{"type": ..., "uid": ...} 만 남깁니다."""
    if isinstance(ds, dict):
        return pick(ds, ["type", "uid"])
    return ds


def thresholds(th):
    """grafanalib 의 {color, index, value: "null", op, yaxis, line} → {color, value}."""
    if not th or not th.get("steps"):
        return None  # step 이 없는 임계값 = 임계값 없음
    steps = []
    for s in th.get("steps") or []:
        value = s.get("value")
        steps.append({"color": s.get("color"), "value": None if value in (None, "null") else value})
    return {"mode": th.get("mode"), "steps": steps}


def pick(src: dict, keys) -> dict:
    return {k: src[k] for k in keys if k in src}


def dig(src, path):
    for k in path:
        if not isinstance(src, dict):
            return None
        src = src.get(k)
    return src


def clean(v):
    """null, false, "", 빈 목록/객체를 제거합니다. Grafana 는 이 값들을 "필드 없음"과 똑같이 취급합니다."""
    if isinstance(v, dict):
        out = {}
        for k, val in v.items():
            c = clean(val)
            if c in (None, False, "", [], {}):
                continue
            out[k] = c
        return out
    if isinstance(v, list):
        return [clean(x) for x in v]
    if isinstance(v, float) and v.is_integer():
        return int(v)  # 1.0 과 1 을 같게
    return v


# ---------------------------------------------------------------------------
# diff
# ---------------------------------------------------------------------------


def diff(path: str, left, right) -> list:
    if isinstance(left, dict) and isinstance(right, dict):
        out = []
        for k in sorted(set(left) | set(right)):
            out += diff(f"{path}.{k}" if path else k, left.get(k), right.get(k))
        return out
    if isinstance(left, list) and isinstance(right, list) and len(left) == len(right):
        out = []
        for i, (lv, rv) in enumerate(zip(left, right)):
            out += diff(f"{path}[{i}]", lv, rv)
        return out
    if left == right:
        return []
    return [f"  {path}\n    - {dump(left)}\n    + {dump(right)}"]


def dump(v) -> str:
    return "(없음)" if v is None else json.dumps(v, ensure_ascii=False)


if __name__ == "__main__":
    sys.stdout.reconfigure(encoding="utf-8")
    sys.exit(main())
