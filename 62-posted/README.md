# 62-posted — grafanalib vs Grafana Foundation SDK

같은 대시보드를 **grafanalib(Python)**, **Foundation SDK(Python)**, **Foundation SDK(Go)** 세 가지로 만들고, 서로 **마이그레이션**하는 방법을 정리한 예제입니다.

```
                  ┌───────────── 같은 언어 (Python) ─────────────┐
grafanalib  ◀────────────────────────────────────────────▶  Foundation SDK (Python)
(Python)    ◀──────────── 다른 언어 ────────────┐                     ▲
                                              ▼                     │ 같은 SDK, 다른 언어
                                     Foundation SDK (Go)  ◀──────────┘

         모든 화살표의 판정 기준: 생성된 JSON 을 tools/compare.py 로 비교
```

| 비교 | 결과 |
| --- | --- |
| grafanalib ↔ SDK Python | ✅ 의미 있는 필드가 모두 같음 |
| grafanalib ↔ SDK Go | ✅ 의미 있는 필드가 모두 같음 |
| SDK Python ↔ SDK Go | ✅ **JSON 파일이 바이트 단위로 완전히 같음** |
| Grafana 12.0.1 업로드 | ✅ 세 결과물 모두 11개 패널, 화면 동일 |

---

## 목차

1. [한눈에 비교](#1-한눈에-비교)
2. [블로그 글 구성 제안](#2-블로그-글-구성-제안)
3. [예제 구성](#3-예제-구성)
4. [준비 — Kiro IDE에서 열기](#4-준비--kiro-ide에서-열기)
5. [실행](#5-실행)
6. [문법 비교](#6-문법-비교)
7. [마이그레이션](#7-마이그레이션)
8. [직접 해보며 찾은 함정들](#8-직접-해보며-찾은-함정들)
9. [무엇을 고를까](#9-무엇을-고를까)
10. [참고 자료](#10-참고-자료)

---

## 1. 한눈에 비교

| 항목 | grafanalib | Grafana Foundation SDK |
| --- | --- | --- |
| 만든 곳 | Weaveworks (커뮤니티 오픈소스) | **Grafana Labs 공식** |
| 최신 릴리스 | 0.7.1 (**2024-01**) | 0.0.20 (2026-09) |
| 언어 | Python | Go, TypeScript, Python, Java, PHP |
| 모델 생성 방식 | 사람이 직접 작성한 attrs 클래스 | Grafana 스키마에서 **자동 생성** (새 패널·옵션이 바로 반영) |
| 문법 | 생성자에 키워드 인자 | 빌더 + 메서드 체이닝 |
| 레이아웃 | `GridPos(h, w, x, y)` 직접 계산 | `span` / `height`만 주면 **자동 배치** |
| 지원 안 되는 옵션 | `extraJson`으로 JSON 덮어쓰기 | 대부분 빌더 메서드가 있음 |
| 출력 JSON | 레거시 필드·기본값까지 **전부** 출력 (28KB) | **지정한 것만** 출력 (14KB) |
| 패널 ID | `.auto_panel_ids()` 제공 | 없음 (직접 후처리) |
| 기존 JSON → 코드 변환 | 없음 | **Go만** (`DashboardConverter`) |
| Dashboard v2 스키마 | 없음 | `dashboardv2beta1`, `dashboardv2` |
| 코드 줄 수 (이 예제) | 280줄 | Python 246줄 / Go 227줄 |

> grafanalib은 PyPI 마지막 릴리스가 2024년 1월(0.7.1)입니다. GitHub 저장소가 아카이브되지는 않았지만, Grafana 10~12에서 추가된 옵션은 클래스에 반영되지 않은 게 많아서 `extraJson`에 의존하게 됩니다.

---

## 2. 블로그 글 구성 제안

1. **왜 비교하는가:** 대시보드를 코드로 관리하는 두 갈래(커뮤니티 grafanalib vs 공식 Foundation SDK)
2. **같은 대시보드를 세 번 만들기:** [SPEC.md](SPEC.md) 명세 하나로 grafanalib / SDK Python / SDK Go 구현
3. **문법 비교:** [6장](#6-문법-비교)의 코드 나란히 보기
4. **출력 JSON 비교:** 28KB vs 14KB, 레거시 필드와 기본값의 함정 ([8장](#8-직접-해보며-찾은-함정들))
5. **마이그레이션 4방향:** 같은 언어 / 다른 언어 / 역방향 / 같은 SDK 다른 언어 ([7장](#7-마이그레이션))
6. **검증 방법:** "같은 대시보드"를 어떻게 증명하는가 (`compare.py`, Grafana 업로드)
7. **결론:** 언제 무엇을 쓸까 ([9장](#9-무엇을-고를까))

---

## 3. 예제 구성

```
62-posted/
├── SPEC.md                               # 대시보드 명세 (세 구현의 기준)
├── tasks.py                              # 모든 작업의 진입점 (Windows / macOS / Linux 공통)
├── requirements.txt                      # grafanalib==0.7.1, grafana-foundation-sdk==0.0.20
├── grafanalib/
│   └── node_overview.dashboard.py        # ① grafanalib
├── foundation_sdk_python/
│   └── node_overview.py                  # ② Foundation SDK (Python)
├── foundation_sdk_go/
│   ├── main.go                           # ③ Foundation SDK (Go)
│   └── cmd/convert/main.go               # JSON → Go 코드 초안 변환기
├── tools/
│   ├── compare.py                        # 대시보드 JSON 비교 (의미 비교 / --exact)
│   └── normalize_grafanalib.py           # grafanalib JSON 정리 (SDK 변환기 입력용)
├── dist/                                 # 생성 결과 (커밋 대상, 직접 수정 금지)
│   ├── grafanalib.json
│   ├── grafanalib.normalized.json
│   ├── fsdk-python.json
│   └── fsdk-go.json
├── docker-compose.yaml                   # 로컬 Grafana 12.0.1 (+ Prometheus / Loki 데이터소스)
├── .vscode/                              # Kiro / VS Code 작업(Tasks), 추천 확장
└── .kiro/steering/sdk-compare.md         # Kiro 에이전트용 프로젝트 설명
```

---

## 4. 준비 — Kiro IDE에서 열기

### 4-1. 설치할 것

| 도구 | 버전 | 확인 |
| --- | --- | --- |
| Python | **3.11 이상** (Foundation SDK Python 요구사항) | `py -3 --version` / `python3 --version` |
| Go | 1.24 이상 | `go version` |
| Docker Desktop | 로컬 Grafana를 띄울 때만 | `docker compose version` |

Windows:
```powershell
winget install Python.Python.3.12
winget install GoLang.Go
winget install Docker.DockerDesktop
```

### 4-2. Kiro에서 열기

1. 레포를 받습니다.
   ```powershell
   git clone https://github.com/High-PO/Grafana-Dashboard-Examples.git
   cd Grafana-Dashboard-Examples
   git checkout feat/62-grafanalib-vs-foundation-sdk   # main 에 합치기 전이라면
   ```
2. Kiro → **File → Open Folder** → **`62-posted` 폴더**를 엽니다. 레포 루트가 아니라 62-posted 폴더를 열어야 아래 설정이 적용됩니다.
   - `.vscode/tasks.json`: Kiro는 VS Code(Code OSS) 기반이라 그대로 인식합니다.
   - `.vscode/extensions.json`: Python, Go 확장 설치를 안내합니다.
   - `.kiro/steering/sdk-compare.md`: Kiro 에이전트(채팅)가 이 프로젝트의 구조와 규칙을 알고 시작합니다.
3. `Ctrl+Shift+P` → **Tasks: Run Task** → **`62: setup (가상환경 + 패키지 설치)`**
4. 같은 메뉴에서 **`62: build`** → **`62: compare`** 순서로 실행합니다.

### 4-3. 터미널로 직접 설정할 경우

```powershell
# Windows PowerShell
cd 62-posted
py -3 -m venv .venv
.venv\Scripts\python.exe -m pip install -r requirements.txt
.venv\Scripts\python.exe tasks.py build
```

```bash
# macOS / Linux
cd 62-posted
python3 -m venv .venv
.venv/bin/python -m pip install -r requirements.txt
.venv/bin/python tasks.py build
```

> ⚠️ `pip install grafana-foundation-sdk`처럼 **버전 없이 설치하면 안 됩니다.** PyPI에 epoch가 붙은 옛 빌드(`1769699998!10.1.0`)가 있어서 0.0.20 대신 Grafana 10.1용 빌드가 설치됩니다. `requirements.txt`처럼 `==0.0.20`으로 고정하세요.

---

## 5. 실행

모든 작업은 `tasks.py` 하나로 합니다. Kiro의 **Tasks: Run Task** 메뉴에도 같은 이름으로 등록돼 있어요.

| 명령 (`python tasks.py ...`) | 하는 일 |
| --- | --- |
| `build` | 세 구현으로 `dist/*.json` 생성 + grafanalib JSON 정리본 생성 |
| `compare` | 4가지 비교 실행 (아래) |
| `check` | `build` + `compare` + `dist/`가 커밋된 내용과 같은지 (CI와 동일) |
| `convert` | grafanalib JSON → 정리 → Foundation SDK **Go 코드 초안** (`dist/grafanalib-to-go.draft.txt`) |
| `up` / `down` | 로컬 Grafana 12.0.1 실행 / 종료 |
| `smoke` | 세 결과물을 UID만 바꿔서 Grafana에 **나란히** 업로드 (`--cleanup` 주면 확인 후 삭제) |

```
$ python tasks.py compare
==> grafanalib → SDK Python (같은 언어)
OK: 의미 있는 필드가 모두 같습니다.  (dist/grafanalib.json == dist/fsdk-python.json)
==> grafanalib → SDK Go (다른 언어)
OK: 의미 있는 필드가 모두 같습니다.  (dist/grafanalib.json == dist/fsdk-go.json)
==> SDK Python ↔ SDK Go (완전 일치)
OK: JSON 전체가 같습니다.  (dist/fsdk-python.json == dist/fsdk-go.json)
==> grafanalib 원본 ↔ 정리본
OK: 의미 있는 필드가 모두 같습니다.  (dist/grafanalib.json == dist/grafanalib.normalized.json)
```

### Grafana에서 눈으로 비교하기

```powershell
.venv\Scripts\python.exe tasks.py up
.venv\Scripts\python.exe tasks.py smoke
# → http://localhost:3000 의 "62 grafanalib vs Foundation SDK" 폴더에 3개 대시보드
```

홈랩 Prometheus / Loki를 port-forward 해두면 실제 데이터로 볼 수 있습니다. 연결하지 않으면 패널에 빨간 경고와 "No data"가 나오는데, 이는 정상입니다.

```powershell
kubectl -n monitoring port-forward svc/<prometheus> 9090:9090
kubectl -n monitoring port-forward svc/<loki>       3100:3100
```

---

## 6. 문법 비교

같은 패널을 세 가지로 쓰면 이렇게 됩니다. 전체 코드는 각 폴더에 있습니다.

### 대시보드

```python
# grafanalib
Dashboard(
    title="Node Overview",
    uid="node-overview",
    timezone="browser",          # 기본값 "utc"
    graphTooltip=1,
    schemaVersion=41,            # 기본값 12 (!)
    templating=Templating(list=[...]),
    panels=[...],
).auto_panel_ids()
```

```python
# Foundation SDK (Python)
dashboard.Dashboard("Node Overview") \
    .uid("node-overview") \
    .tooltip(DashboardCursorSync.CROSSHAIR) \
    .with_variable(...) \
    .with_row(dashboard.Row("요약")) \
    .with_panel(...)
```

```go
// Foundation SDK (Go)
dashboard.NewDashboardBuilder("Node Overview").
    Uid("node-overview").
    Tooltip(dashboard.DashboardCursorSyncCrosshair).
    WithVariable(...).
    WithRow(dashboard.NewRowBuilder("요약")).
    WithPanel(...)
```

### Stat 패널 (min/max 포함)

```python
# grafanalib — Stat 에는 min / max 속성이 없어서 extraJson 으로 JSON 을 직접 덮어씀
Stat(
    title="CPU 사용률",
    dataSource={"type": "prometheus", "uid": "${datasource}"},
    targets=[Target(expr=CPU_USAGE, refId="A", datasource=PROMETHEUS, intervalFactor=1)],
    gridPos=GridPos(h=4, w=6, x=6, y=1),      # 좌표 직접 계산
    format="percentunit",                      # 단위가 "format"
    decimals=1,
    reduceCalc="lastNotNull",
    thresholds=[
        Threshold(color="green", index=0, value=0.0),
        Threshold(color="orange", index=1, value=0.7),
        Threshold(color="red", index=2, value=0.9),
    ],
    maxDataPoints=None, noValue=None,          # 기본값 끄기 (8장 참고)
    extraJson={"fieldConfig": {"defaults": {"min": 0, "max": 1}}},
)
```

```python
# Foundation SDK (Python)
stat.Panel() \
    .title("CPU 사용률") \
    .datasource(PROMETHEUS) \
    .with_target(prometheus.Dataquery().expr(CPU_USAGE).ref_id("A")) \
    .unit("percentunit") \
    .span(6).height(4) \
    .decimals(1).min(0).max(1) \
    .reduce_options(ReduceDataOptions().calcs(["lastNotNull"])) \
    .thresholds(ThresholdsConfig().mode(ABSOLUTE).steps([
        Threshold(color="green"),
        Threshold(value=0.7, color="orange"),
        Threshold(value=0.9, color="red"),
    ]))
```

```go
// Foundation SDK (Go)
stat.NewPanelBuilder().
    Title("CPU 사용률").
    Datasource(promDS).
    WithTarget(prometheus.NewDataqueryBuilder().Expr(cpuUsage).RefId("A")).
    Unit("percentunit").
    Span(6).Height(4).
    Decimals(1).Min(0).Max(1).
    ReduceOptions(common.NewReduceDataOptionsBuilder().Calcs([]string{"lastNotNull"})).
    Thresholds(dashboard.NewThresholdsConfigBuilder().Mode(dashboard.ThresholdsModeAbsolute).Steps(
        []dashboard.Threshold{{Color: "green"}, {Value: cog.ToPtr(0.7), Color: "orange"}, {Value: cog.ToPtr(0.9), Color: "red"}}))
```

### Override (특정 시리즈만 아래로)

```python
# grafanalib — dict 를 그대로
overrides=[{
    "matcher": {"id": "byRegexp", "options": ".* tx$"},
    "properties": [{"id": "custom.transform", "value": "negative-Y"}],
}]
```

```python
# Foundation SDK (Python)
.override_by_regexp(".* tx$", [DynamicConfigValue(id_val="custom.transform", value="negative-Y")])
```

```go
// Foundation SDK (Go)
OverrideByRegexp(".* tx$", []dashboard.DynamicConfigValue{{Id: "custom.transform", Value: "negative-Y"}})
```

### JSON 출력

| | 방법 |
| --- | --- |
| grafanalib | `generate-dashboard -o out.json x.dashboard.py` (파일명 `*.dashboard.py` + `dashboard` 변수 관례) |
| SDK Python | `json.dumps(dash, cls=JSONEncoder)` |
| SDK Go | `json.Marshal(dash)` |

---

## 7. 마이그레이션

### 7-0. 공통 원칙

1. **JSON이 공통 언어입니다.** 어떤 라이브러리든 결국 Grafana JSON을 만듭니다. 마이그레이션 전후 JSON을 뽑아서 비교하면 "같은 대시보드인지"를 기계적으로 판정할 수 있습니다.
2. **판정은 `tools/compare.py`가 합니다.** 쿼리, 단위, 임계값, 배치, 변수, 주요 옵션처럼 화면과 동작에 영향을 주는 값만 비교하고, 레거시 필드나 기본값 표기 차이는 무시합니다.
   ```powershell
   python tools/compare.py <마이그레이션 전.json> <마이그레이션 후.json>
   ```
3. **자동 변환기는 "JSON → Go"만 있습니다.** Python SDK와 grafanalib에는 JSON → 코드 변환기가 없습니다. 그래서 다른 방향은 아래 매핑표를 보고 손으로 옮기고, `compare.py`로 검증합니다.

### 매핑표

| 개념 | grafanalib | SDK Python | SDK Go |
| --- | --- | --- | --- |
| 대시보드 | `Dashboard(title=, uid=)` | `dashboard.Dashboard(t).uid()` | `dashboard.NewDashboardBuilder(t).Uid()` |
| 행 | `RowPanel(title=, gridPos=)` | `.with_row(dashboard.Row(t))` | `.WithRow(dashboard.NewRowBuilder(t))` |
| 배치 | `gridPos=GridPos(h, w, x, y)` **필수** | `.span(w).height(h)` | `.Span(w).Height(h)` |
| 데이터소스 변수 | `Template(type="datasource", query="prometheus")` | `DatasourceVariable(n).type("prometheus")` | `NewDatasourceVariableBuilder(n).Type(...)` |
| 쿼리 변수 | `Template(query="label_values(...)", refresh=2, sort=1)` | `QueryVariable(n).query({...}).refresh(ON_TIME_RANGE_CHANGED)` | `NewQueryVariableBuilder(n).Query(StringOrMap{...})` |
| Prometheus 쿼리 | `Target(expr=, legendFormat=, refId=)` | `prometheus.Dataquery().expr().legend_format()` | `prometheus.NewDataqueryBuilder().Expr().LegendFormat()` |
| Loki 쿼리 | `Target(expr=, datasource=LOKI)` (전용 클래스 없음) | `loki.Dataquery().expr()` | `loki.NewDataqueryBuilder().Expr()` |
| 단위 | Stat: `format=` / TimeSeries: `unit=` | `.unit()` | `.Unit()` |
| min / max | TimeSeries: `valueMin=, valueMax=` / Stat: **`extraJson`** | `.min().max()` | `.Min().Max()` |
| 소수점 | Stat: `decimals=` / TimeSeries: `valueDecimals=` | `.decimals()` | `.Decimals()` |
| 임계값 | `[Threshold(color, index, value)]` | `ThresholdsConfig().steps([Threshold(value, color)])` | `NewThresholdsConfigBuilder().Steps(...)` |
| Stat 집계 | `reduceCalc="lastNotNull"` | `.reduce_options(ReduceDataOptions().calcs([...]))` | `.ReduceOptions(...)` |
| 범례 | `legendDisplayMode=, legendPlacement=, legendCalcs=` | `.legend(VizLegendOptions()...)` | `.Legend(...)` |
| 툴팁 | `tooltipMode=, tooltipSort=` | `.tooltip(VizTooltipOptions()...)` | `.Tooltip(...)` |
| Override | `overrides=[dict]` | `.override_by_regexp(re, [DynamicConfigValue])` | `.OverrideByRegexp(re, ...)` |
| 로그 줄바꿈 | `wrapLogMessages=` (**s** 붙음) | `.wrap_log_message()` | `.WrapLogMessage()` |
| 패널 ID | `.auto_panel_ids()` | 빌드 후 직접 부여 | 빌드 후 직접 부여 |
| 미지원 옵션 | `extraJson={...}` | (대부분 빌더 있음) | (대부분 빌더 있음) |

### 7-1. grafanalib → Foundation SDK Python (같은 언어)

자동 변환기가 없어서 손으로 옮기지만, 같은 언어라서 헬퍼 함수와 PromQL 상수는 그대로 가져올 수 있습니다.

1. **기준 JSON 만들기**
   ```powershell
   python tasks.py build          # dist/grafanalib.json
   ```
2. **옮기기:** 위 매핑표를 보면서 패널 단위로 바꿉니다.
   - PromQL 상수(`CPU_USAGE` 등)는 복사해서 그대로 씁니다.
   - `GridPos(h, w, x, y)` → `.span(w).height(h)`. 행 순서대로 `with_panel()` 하면 좌표는 SDK가 계산합니다.
   - `extraJson`으로 넣던 값은 SDK 빌더 메서드로 바꿉니다 (예: Stat의 `min` / `max`).
   - `.auto_panel_ids()` 대신 빌드 후 ID를 부여합니다 (`foundation_sdk_python/node_overview.py`의 `build()` 마지막 부분).
   - **팁:** `python tasks.py convert`로 만든 **Go 초안**을 참고하면 편합니다. Python SDK와 Go SDK는 메서드 이름이 `CamelCase` ↔ `snake_case`로 1:1 대응해서, 어떤 메서드를 써야 하는지 바로 보입니다.
3. **검증**
   ```powershell
   python tools/compare.py dist/grafanalib.json dist/fsdk-python.json
   ```
   차이가 나오면 경로(`panels.[stat] CPU 사용률.defaults.unit` 같은 형식)를 보고 해당 패널을 고칩니다.

### 7-2. grafanalib → Foundation SDK Go (다른 언어)

이 방향은 **자동 변환기를 쓸 수 있습니다.** 다만 grafanalib JSON을 바로 넣으면 실패해서, 정리 단계가 하나 필요합니다.

```
grafanalib.json ──normalize──▶ grafanalib.normalized.json ──convert──▶ Go 코드 초안 ──정리──▶ main.go
```

1. **변환**
   ```powershell
   python tasks.py convert
   # = normalize_grafanalib.py + go run ./cmd/convert
   # → dist/grafanalib-to-go.draft.txt (약 370줄)
   ```
   정리 없이 바로 변환하면 이렇게 실패합니다.
   ```
   json: cannot unmarshal string into Go struct field ... thresholds.steps.value of type float64
   ```
   grafanalib이 첫 임계값을 `"value": "null"`(**문자열**)로 출력하기 때문입니다. `normalize_grafanalib.py`가 이것을 JSON `null`로 바꾸고, 레거시 필드(`intervalFactor`, `step`, `query`, `rows`, `style` ...)를 지웁니다. 정리 전후 JSON이 같은 대시보드인지는 `compare`가 확인합니다.
2. **정리:** 초안은 gofmt도 안 된 긴 체인이고 기본값까지 다 들어 있습니다. 그대로 쓰지 말고 `foundation_sdk_go/main.go`처럼 헬퍼(`statPanel`, `timeseriesPanel`)와 상수로 정리합니다. `GridPos`는 지우고 `Span` / `Height`만 남깁니다.
3. **검증**
   ```powershell
   python tools/compare.py dist/grafanalib.json dist/fsdk-go.json
   ```

### 7-3. Foundation SDK → grafanalib (역방향, Python·Go 공통)

실무에서 흔하지는 않습니다. 대부분 grafanalib에서 SDK로 넘어가니까요. 그래도 "SDK로 만든 대시보드를 기존 grafanalib 저장소에 맞춰 넣어야 할 때"를 위한 절차입니다. 이 방향은 변환기가 없습니다.

1. **기준 JSON:** `dist/fsdk-python.json` 또는 `dist/fsdk-go.json` (두 파일은 완전히 같습니다)
2. **옮기기:** 매핑표를 반대로 읽습니다. 신경 쓸 부분은 다음과 같습니다.
   - **좌표 직접 계산:** SDK가 자동으로 계산해준 `gridPos`를 기준 JSON에서 그대로 복사해 `GridPos(h, w, x, y)`로 넣습니다.
   - **grafanalib에 없는 옵션은 `extraJson`:** 예를 들어 Stat의 `min` / `max`, 그 밖에 최신 Grafana 옵션.
   - **grafanalib 기본값 끄기:** `maxDataPoints=None`, Stat의 `noValue=None`, Target의 `intervalFactor=1`, Dashboard의 `schemaVersion` / `timezone` 지정 (자세한 이유는 8장).
3. **검증**
   ```powershell
   python tools/compare.py dist/fsdk-python.json dist/grafanalib.json
   ```

### 7-4. Foundation SDK Python ↔ Go (같은 SDK, 다른 언어)

가장 쉬운 방향입니다. 같은 스키마에서 생성된 SDK라서 **이름 규칙만 바꾸면 됩니다.**

| Go | Python |
| --- | --- |
| `dashboard.NewDashboardBuilder("t")` | `dashboard.Dashboard("t")` |
| `stat.NewPanelBuilder()` | `stat.Panel()` |
| `prometheus.NewDataqueryBuilder()` | `prometheus.Dataquery()` |
| `.WithTarget()`, `.RefId()`, `.LegendFormat()` | `.with_target()`, `.ref_id()`, `.legend_format()` |
| `common.LegendDisplayModeTable` | `common.LegendDisplayMode.TABLE` |
| `dashboard.Threshold{Value: cog.ToPtr(0.7), Color: "orange"}` | `Threshold(value=0.7, color="orange")` |
| `common.DataSourceRef{Type: cog.ToPtr("prometheus")}` | `common.DataSourceRef(type_val="prometheus")` |
| `Id` 필드 | `id_val` (Python 예약어 `id`, `type` 회피) |

검증은 **완전 일치(`--exact`)**로 합니다. 같은 SDK라면 JSON이 한 글자도 다르지 않아야 합니다.

```powershell
python tools/compare.py --exact dist/fsdk-python.json dist/fsdk-go.json
```

> 처음에는 이 비교가 **실패**했습니다. 8장의 "enum 기본값이 언어마다 다름" 참고.

---

## 8. 직접 해보며 찾은 함정들

| # | 함정 | 증상 | 해결 |
| --- | --- | --- | --- |
| 1 | **PyPI epoch 버전** | `pip install grafana-foundation-sdk`가 0.0.20이 아니라 `1769699998!10.1.0`(Grafana 10.1용)을 설치 | `==0.0.20`으로 고정 |
| 2 | grafanalib `schemaVersion` 기본값 **12** | 아주 오래된 스키마로 인식되어 Grafana가 마이그레이션 시도 | `schemaVersion=41` 명시 |
| 3 | grafanalib `timezone` 기본값 `utc` | UI로 만든 대시보드(`browser`)와 시간 표시가 다름 | `timezone="browser"` |
| 4 | grafanalib Stat `noValue` 기본값 `"none"` | 데이터가 없으면 화면에 **"none" 글자**가 그대로 보임 | `noValue=None` |
| 5 | grafanalib `maxDataPoints` 기본값 **100** | 모든 패널의 그래프 해상도가 100포인트로 제한 | `maxDataPoints=None` |
| 6 | grafanalib Target `intervalFactor` 기본값 2 | 레거시 해상도 설정이 붙어서 쿼리 step에 영향을 줄 수 있음 | `intervalFactor=1` |
| 7 | grafanalib 임계값 `"value": "null"` (문자열) | Foundation SDK(Go) 변환기가 읽지 못함 | `tools/normalize_grafanalib.py` |
| 8 | grafanalib Stat에 `min` / `max` 없음 | 옵션 누락 | `extraJson` |
| 9 | grafanalib `gridPos` 수동 | 패널 하나를 추가하면 아래 좌표를 전부 다시 계산 | SDK는 `span` / `height`로 자동 배치 |
| 10 | **SDK enum 기본값이 언어마다 다름** | 지정하지 않으면 Python은 첫 번째 값(`orientation: "auto"`, `dedupStrategy: "none"`), Go는 `""` → Python ↔ Go JSON 불일치 | enum 옵션은 **항상 명시** |
| 11 | SDK `schemaVersion` 기본값 42 | Grafana 12.0.x 저장값(41)과 달라 불필요한 diff | 빌드 후 41로 덮어쓰기 |
| 12 | SDK 패널 ID 없음 | `viewPanel=` 링크·패널 복제에 필요 | 빌드 후 순서대로 부여 |
| 13 | Python SDK에 JSON → 코드 변환기 없음 | 기존 JSON을 옮길 때 초안이 없음 | Go 변환기 초안을 보고 snake_case로 옮기기 |
| 14 | Windows PowerShell 5.1의 `>` | 리다이렉트한 JSON이 UTF-16으로 저장되어 깨짐 | `tasks.py`가 UTF-8로 직접 저장 |

1~9번은 **grafanalib을 쓰는 동안에는 잘 드러나지 않다가**, 다른 도구로 옮기거나 출력 JSON을 비교할 때 비로소 보이는 것들입니다. 마이그레이션 전후로 JSON 비교를 꼭 해야 하는 이유이기도 합니다.

---

## 9. 무엇을 고를까

| 상황 | 추천 |
| --- | --- |
| 새로 시작 | **Foundation SDK.** 공식 지원, 스키마 자동 생성, 자동 배치, v2 스키마 대비 |
| 팀이 Python 중심 | Foundation SDK **Python** (`==0.0.20` 고정 필수) |
| 기존 JSON이 많아서 옮겨야 함 | Foundation SDK **Go.** JSON → 코드 변환기가 Go에만 있음 |
| grafanalib 코드가 이미 많음 | 당장 문제없으면 유지하되, 8장의 2~6번 기본값은 바로 점검. 새 대시보드부터 SDK로 작성하고 7-1 / 7-2로 점진 이전 |
| Terraform으로 배포 | 어느 쪽이든 JSON을 만들면 `grafana_dashboard`에 그대로 넣을 수 있음 ([60-posted](../60-posted) 참고) |

---

## 10. 참고 자료

- [Grafana Foundation SDK 문서](https://grafana.com/docs/grafana/latest/as-code/observability-as-code/foundation-sdk/) · [GitHub](https://github.com/grafana/grafana-foundation-sdk)
- [grafana-foundation-sdk (PyPI)](https://pypi.org/project/grafana-foundation-sdk/)
- [grafanalib GitHub](https://github.com/weaveworks/grafanalib) · [PyPI](https://pypi.org/project/grafanalib/) · [Releases](https://github.com/weaveworks/grafanalib/releases)
- [Kiro](https://kiro.dev/)
