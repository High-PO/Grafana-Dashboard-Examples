"""62-posted 작업 실행기 (Windows / macOS / Linux 공통).

Makefile 대신 Python 으로 만들었습니다. Windows PowerShell 에서도 그대로 동작합니다.

    python tasks.py build      # 세 가지 구현으로 dist/*.json 생성
    python tasks.py compare    # 결과 비교 (grafanalib ↔ SDK 의미 비교, SDK Python ↔ Go 완전 일치)
    python tasks.py check      # build + compare + dist/ 가 커밋된 것과 같은지 (CI 와 동일)
    python tasks.py convert    # grafanalib JSON → 정리 → Foundation SDK Go 코드 초안
    python tasks.py up         # 로컬 Grafana 12.0.1 실행 (docker compose)
    python tasks.py smoke      # 세 결과물을 Grafana 에 나란히 올려서 확인
    python tasks.py down       # 로컬 Grafana 종료

PowerShell 의 `>` 리다이렉트는 (5.1 버전에서) UTF-16 으로 저장되어 JSON 이 깨집니다.
그래서 이 스크립트는 출력을 항상 UTF-8 파일로 직접 씁니다.
"""

import argparse
import base64
import json
import os
import shutil
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent
DIST = ROOT / "dist"
GO_DIR = ROOT / "foundation_sdk_go"

GRAFANALIB_SRC = ROOT / "grafanalib" / "node_overview.dashboard.py"
OUT_GRAFANALIB = DIST / "grafanalib.json"
OUT_NORMALIZED = DIST / "grafanalib.normalized.json"
OUT_PYTHON = DIST / "fsdk-python.json"
OUT_GO = DIST / "fsdk-go.json"
OUT_DRAFT = DIST / "grafanalib-to-go.draft.txt"

GRAFANA_URL = os.environ.get("GRAFANA_URL", "http://localhost:3000")
GRAFANA_AUTH = os.environ.get("GRAFANA_AUTH", "admin:admin")


def log(msg: str) -> None:
    print(f"==> {msg}", flush=True)


def run(cmd, cwd=None, capture=False) -> bytes:
    """명령 실행. 실패하면 바로 종료합니다."""
    result = subprocess.run(cmd, cwd=cwd, stdout=subprocess.PIPE if capture else None)
    if result.returncode != 0:
        sys.exit(f"실패: {' '.join(map(str, cmd))}")
    return result.stdout or b""


def write_json_bytes(path: Path, data: bytes) -> None:
    """JSON 을 다시 읽어서 sort_keys + indent=2 로 저장합니다. (생성기마다 다른 키 순서/공백을 통일)"""
    obj = json.loads(data.decode("utf-8"))
    path.write_text(json.dumps(obj, indent=2, ensure_ascii=False, sort_keys=True) + "\n", encoding="utf-8")
    print(f"    generated {path.relative_to(ROOT)}")


# ---------------------------------------------------------------------------
# build / compare / check
# ---------------------------------------------------------------------------


def build_grafanalib() -> None:
    """grafanalib: *.dashboard.py 의 dashboard 변수를 읽어서 JSON 으로 (generate-dashboard 와 동일)."""
    from grafanalib._gen import DashboardEncoder, loader

    dash = loader(str(GRAFANALIB_SRC))
    data = json.dumps(dash.to_json_data(), cls=DashboardEncoder).encode("utf-8")
    write_json_bytes(OUT_GRAFANALIB, data)


def build_fsdk_python() -> None:
    sys.path.insert(0, str(ROOT / "foundation_sdk_python"))
    import node_overview

    write_json_bytes(OUT_PYTHON, node_overview.to_json(node_overview.build()).encode("utf-8"))


def build_fsdk_go() -> None:
    if not shutil.which("go"):
        sys.exit("go 명령을 찾을 수 없습니다. Go 1.24 이상을 설치하세요.")
    write_json_bytes(OUT_GO, run(["go", "run", "."], cwd=GO_DIR, capture=True))


def cmd_build(_args) -> None:
    DIST.mkdir(exist_ok=True)
    log("grafanalib")
    build_grafanalib()
    log("Foundation SDK (Python)")
    build_fsdk_python()
    log("Foundation SDK (Go)")
    build_fsdk_go()
    log("grafanalib JSON 정리 (SDK 변환기 입력용)")
    run([sys.executable, "-I", str(ROOT / "tools" / "normalize_grafanalib.py"), str(OUT_GRAFANALIB), str(OUT_NORMALIZED)])


def compare(*args: str) -> bool:
    result = subprocess.run([sys.executable, "-I", str(ROOT / "tools" / "compare.py"), *args])
    return result.returncode == 0


def cmd_compare(_args) -> None:
    rel = lambda p: str(p.relative_to(ROOT))  # noqa: E731
    os.chdir(ROOT)
    checks = [
        ("grafanalib → SDK Python (같은 언어)", [rel(OUT_GRAFANALIB), rel(OUT_PYTHON)]),
        ("grafanalib → SDK Go (다른 언어)", [rel(OUT_GRAFANALIB), rel(OUT_GO)]),
        ("SDK Python ↔ SDK Go (완전 일치)", ["--exact", rel(OUT_PYTHON), rel(OUT_GO)]),
        ("grafanalib 원본 ↔ 정리본", [rel(OUT_GRAFANALIB), rel(OUT_NORMALIZED)]),
    ]
    failed = 0
    for title, argv in checks:
        log(title)
        if not compare(*argv):
            failed += 1
    if failed:
        sys.exit(f"{failed}개 비교 실패")


def cmd_check(args) -> None:
    cmd_build(args)
    cmd_compare(args)
    if shutil.which("git"):
        log("dist/ 가 커밋된 내용과 같은지")
        status = subprocess.run(
            ["git", "status", "--porcelain", "--", "dist"], cwd=ROOT, stdout=subprocess.PIPE, text=True
        ).stdout.strip()
        if status:
            print(status)
            sys.exit("dist/ 가 코드와 다릅니다. 'python tasks.py build' 결과를 함께 커밋하세요.")
        print("    OK")


def cmd_convert(_args) -> None:
    """grafanalib → (정리) → Foundation SDK Go 코드 초안."""
    if not OUT_GRAFANALIB.exists():
        cmd_build(_args)
    run([sys.executable, "-I", str(ROOT / "tools" / "normalize_grafanalib.py"), str(OUT_GRAFANALIB), str(OUT_NORMALIZED)])
    run(["go", "run", "./cmd/convert", "-o", str(OUT_DRAFT), str(OUT_NORMALIZED)], cwd=GO_DIR)
    log(f"초안 생성: {OUT_DRAFT.relative_to(ROOT)} ({len(OUT_DRAFT.read_bytes().splitlines())}줄)")
    print("    Go: 그대로 정리해서 사용 / Python: 메서드 이름만 snake_case 로 바꾸면 거의 1:1 로 대응합니다.")


# ---------------------------------------------------------------------------
# 로컬 Grafana
# ---------------------------------------------------------------------------


def cmd_up(_args) -> None:
    run(["docker", "compose", "up", "-d", "--wait"], cwd=ROOT)
    print(f"    {GRAFANA_URL} (admin / admin)")


def cmd_down(_args) -> None:
    run(["docker", "compose", "down", "-v"], cwd=ROOT)


def api(method: str, path: str, body=None):
    req = urllib.request.Request(GRAFANA_URL + path, method=method)
    req.add_header("Authorization", "Basic " + base64.b64encode(GRAFANA_AUTH.encode()).decode())
    req.add_header("Content-Type", "application/json")
    data = json.dumps(body).encode("utf-8") if body is not None else None
    try:
        with urllib.request.urlopen(req, data=data, timeout=10) as res:
            return res.status, json.loads(res.read() or b"{}")
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read() or b"{}")


def count_panels(dash: dict) -> int:
    return sum(1 + len(p.get("panels") or []) for p in dash.get("panels") or [])


def cmd_smoke(args) -> None:
    """세 결과물을 UID 만 바꿔서 같은 폴더에 올립니다. Grafana 에서 나란히 열어 비교할 수 있습니다."""
    folder = "sdk-compare"
    api("POST", "/api/folders", {"uid": folder, "title": "62 grafanalib vs Foundation SDK"})

    failed = 0
    for suffix, path in [("grafanalib", OUT_GRAFANALIB), ("fsdk-python", OUT_PYTHON), ("fsdk-go", OUT_GO)]:
        dash = json.loads(path.read_text(encoding="utf-8"))
        dash["uid"] = f"{dash['uid']}-{suffix}"
        dash["title"] = f"{dash['title']} ({suffix})"
        dash.pop("id", None)

        status, res = api("POST", "/api/dashboards/db", {"dashboard": dash, "folderUid": folder, "overwrite": True})
        if status != 200:
            print(f"FAIL  {suffix}: {status} {res}")
            failed += 1
            continue

        status, saved = api("GET", f"/api/dashboards/uid/{dash['uid']}")
        got, want = count_panels(saved.get("dashboard", {})), count_panels(dash)
        if status != 200 or got != want:
            print(f"FAIL  {suffix}: panels={got} (기대값 {want})")
            failed += 1
            continue
        print(f"OK    {suffix:12} {got} panels → {GRAFANA_URL}{res.get('url')}")

    if args.cleanup:
        api("DELETE", f"/api/folders/{folder}")
        print("    정리 완료 (폴더와 대시보드 삭제)")

    if failed:
        sys.exit(1)


def main() -> None:
    sys.stdout.reconfigure(encoding="utf-8")

    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("build", help="dist/*.json 생성").set_defaults(func=cmd_build)
    sub.add_parser("compare", help="결과 비교").set_defaults(func=cmd_compare)
    sub.add_parser("check", help="build + compare + dist 최신 여부").set_defaults(func=cmd_check)
    sub.add_parser("convert", help="grafanalib JSON → Go 코드 초안").set_defaults(func=cmd_convert)
    sub.add_parser("up", help="로컬 Grafana 실행").set_defaults(func=cmd_up)
    sub.add_parser("down", help="로컬 Grafana 종료").set_defaults(func=cmd_down)
    smoke = sub.add_parser("smoke", help="Grafana 에 올려서 확인")
    smoke.add_argument("--cleanup", action="store_true", help="확인 후 삭제")
    smoke.set_defaults(func=cmd_smoke)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
