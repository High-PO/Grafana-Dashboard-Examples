"""Node Overview — grafanalib 버전.

grafanalib 의 관례대로 파일 이름이 `*.dashboard.py` 이고, 모듈 안에 `dashboard` 변수를 정의합니다.
`generate-dashboard` CLI 가 이 변수를 찾아서 JSON 으로 출력합니다.

    generate-dashboard -o ../dist/grafanalib.json node_overview.dashboard.py

같은 대시보드를 Foundation SDK(Python / Go)로 만든 코드와 나란히 비교하기 위한 예제입니다.
명세는 ../SPEC.md 를 참고하세요.
"""

from grafanalib.core import (
    Dashboard,
    GridPos,
    Logs,
    RowPanel,
    Stat,
    Target,
    Template,
    Templating,
    Threshold,
    Time,
    TimeSeries,
)

# ---------------------------------------------------------------------------
# 공통 값
# ---------------------------------------------------------------------------

# 대상 Grafana(12.0.x)가 저장하는 schemaVersion. grafanalib 의 기본값은 12 라서 꼭 지정해야 합니다.
SCHEMA_VERSION = 41

PROMETHEUS = {"type": "prometheus", "uid": "${datasource}"}
LOKI = {"type": "loki", "uid": "${loki}"}

INSTANCE = 'instance=~"$instance"'


def prom(expr, legend="", ref_id="A"):
    """Prometheus 쿼리.

    grafanalib 의 Target 은 intervalFactor=2, step=10 같은 레거시 필드를 기본으로 넣습니다.
    intervalFactor 는 쿼리 해상도(step)에 영향을 줄 수 있으므로 1 로 맞춥니다.
    """
    return Target(
        expr=expr,
        legendFormat=legend,
        refId=ref_id,
        datasource=PROMETHEUS,
        intervalFactor=1,
    )


def thresholds(*steps):
    """(색상, 값) 목록 → grafanalib Threshold 목록. 첫 단계는 값 없이(-∞) 시작합니다."""
    return [
        Threshold(color=color, index=i, value=float(value or 0))
        for i, (color, value) in enumerate(steps)
    ]


# grafanalib 은 모든 패널에 maxDataPoints=100 을 넣습니다. 그래프 해상도가 100 포인트로 제한되므로 끕니다(None).
NO_MAX_DATA_POINTS = None

PERCENT_THRESHOLDS = thresholds(("green", None), ("orange", 0.7), ("red", 0.9))
DISK_THRESHOLDS = thresholds(("green", None), ("orange", 0.8), ("red", 0.9))


def stat(title, expr, grid, unit="none", decimals=None, steps=None, min_max=None, description=None):
    """요약 Stat 패널.

    grafanalib 의 Stat 에는 min / max 속성이 없어서 extraJson 으로 JSON 을 직접 덮어씁니다.
    (grafanalib 이 지원하지 않는 옵션은 전부 이 방법을 써야 합니다)
    """
    extra = None
    if min_max is not None:
        extra = {"fieldConfig": {"defaults": {"min": min_max[0], "max": min_max[1]}}}

    return Stat(
        title=title,
        description=description,
        dataSource=PROMETHEUS,
        targets=[prom(expr)],
        gridPos=grid,
        format=unit,
        decimals=decimals,
        colorMode="value",
        graphMode="area",
        reduceCalc="lastNotNull",
        thresholds=steps or thresholds(("blue", None)),
        maxDataPoints=NO_MAX_DATA_POINTS,
        # 기본값 "none" 이면 데이터가 없을 때 화면에 "none" 글자가 그대로 보입니다.
        noValue=None,
        extraJson=extra,
    )


def timeseries(title, targets, grid, unit, min_max=None, overrides=None):
    """추이 그래프. 범례는 표 형태(하단), 툴팁은 전체 시리즈 내림차순."""
    return TimeSeries(
        title=title,
        dataSource=PROMETHEUS,
        targets=targets,
        gridPos=grid,
        unit=unit,
        valueMin=min_max[0] if min_max else None,
        valueMax=min_max[1] if min_max else None,
        drawStyle="line",
        lineWidth=1,
        fillOpacity=10,
        showPoints="never",
        legendDisplayMode="table",
        legendPlacement="bottom",
        legendCalcs=["mean", "max", "lastNotNull"],
        tooltipMode="multi",
        tooltipSort="desc",
        overrides=overrides or [],
        maxDataPoints=NO_MAX_DATA_POINTS,
    )


# ---------------------------------------------------------------------------
# PromQL
# ---------------------------------------------------------------------------

CPU_USAGE = f'1 - avg(rate(node_cpu_seconds_total{{mode="idle", {INSTANCE}}}[$__rate_interval]))'
CPU_USAGE_BY_INSTANCE = (
    f'1 - avg by(instance) (rate(node_cpu_seconds_total{{mode="idle", {INSTANCE}}}[$__rate_interval]))'
)
MEM_USAGE = (
    f"1 - sum(node_memory_MemAvailable_bytes{{{INSTANCE}}}) / sum(node_memory_MemTotal_bytes{{{INSTANCE}}})"
)
MEM_USAGE_BY_INSTANCE = (
    f"1 - node_memory_MemAvailable_bytes{{{INSTANCE}}} / node_memory_MemTotal_bytes{{{INSTANCE}}}"
)
ROOT_FS = f'mountpoint="/", fstype!="rootfs", {INSTANCE}'
DISK_USAGE = f"1 - sum(node_filesystem_avail_bytes{{{ROOT_FS}}}) / sum(node_filesystem_size_bytes{{{ROOT_FS}}})"
NET_DEVICE = f'device!~"lo|veth.*|cali.*|flannel.*|cni.*", {INSTANCE}'

# ---------------------------------------------------------------------------
# 대시보드
# ---------------------------------------------------------------------------

dashboard = Dashboard(
    title="Node Overview",
    uid="node-overview",
    description="노드 상태 요약 / 리소스 추이 / 에러 로그. grafanalib 과 Foundation SDK 비교용 예제.",
    tags=["node", "homelab", "sdk-compare"],
    timezone="browser",
    refresh="30s",
    time=Time("now-3h", "now"),
    graphTooltip=1,  # 패널 간 crosshair 공유
    schemaVersion=SCHEMA_VERSION,
    templating=Templating(
        list=[
            Template(
                name="datasource",
                label="Prometheus",
                type="datasource",
                query="prometheus",
            ),
            Template(
                name="loki",
                label="Loki",
                type="datasource",
                query="loki",
            ),
            Template(
                name="instance",
                label="Instance",
                type="query",
                dataSource=PROMETHEUS,
                query="label_values(node_uname_info, instance)",
                refresh=2,  # 시간 범위가 바뀔 때마다
                sort=1,  # 알파벳 오름차순
                multi=True,
                includeAll=True,
            ),
        ]
    ),
    panels=[
        # --- 요약 -----------------------------------------------------------
        RowPanel(title="요약", gridPos=GridPos(h=1, w=24, x=0, y=0)),
        stat(
            "Up",
            f'sum(up{{job=~".*node.*", {INSTANCE}}})',
            GridPos(h=4, w=6, x=0, y=1),
            decimals=0,
            steps=thresholds(("red", None), ("green", 1)),
            description="수집 중인 node-exporter 수",
        ),
        stat(
            "CPU 사용률",
            CPU_USAGE,
            GridPos(h=4, w=6, x=6, y=1),
            unit="percentunit",
            decimals=1,
            steps=PERCENT_THRESHOLDS,
            min_max=(0, 1),
        ),
        stat(
            "Memory 사용률",
            MEM_USAGE,
            GridPos(h=4, w=6, x=12, y=1),
            unit="percentunit",
            decimals=1,
            steps=PERCENT_THRESHOLDS,
            min_max=(0, 1),
        ),
        stat(
            "Root 디스크 사용률",
            DISK_USAGE,
            GridPos(h=4, w=6, x=18, y=1),
            unit="percentunit",
            decimals=1,
            steps=DISK_THRESHOLDS,
            min_max=(0, 1),
        ),
        # --- 리소스 추이 ------------------------------------------------------
        RowPanel(title="리소스 추이", gridPos=GridPos(h=1, w=24, x=0, y=5)),
        timeseries(
            "CPU 사용률 (인스턴스별)",
            [prom(CPU_USAGE_BY_INSTANCE, "{{instance}}")],
            GridPos(h=8, w=12, x=0, y=6),
            unit="percentunit",
            min_max=(0, 1),
        ),
        timeseries(
            "Memory 사용률 (인스턴스별)",
            [prom(MEM_USAGE_BY_INSTANCE, "{{instance}}")],
            GridPos(h=8, w=12, x=12, y=6),
            unit="percentunit",
            min_max=(0, 1),
        ),
        timeseries(
            "Network 송수신",
            [
                prom(
                    f"sum by(instance) (rate(node_network_receive_bytes_total{{{NET_DEVICE}}}[$__rate_interval]))",
                    "{{instance}} rx",
                    "A",
                ),
                prom(
                    f"sum by(instance) (rate(node_network_transmit_bytes_total{{{NET_DEVICE}}}[$__rate_interval]))",
                    "{{instance}} tx",
                    "B",
                ),
            ],
            GridPos(h=8, w=24, x=0, y=14),
            unit="Bps",
            # 송신(tx)은 아래(-)로 그립니다. override 는 dict 를 그대로 넣어야 합니다.
            overrides=[
                {
                    "matcher": {"id": "byRegexp", "options": ".* tx$"},
                    "properties": [{"id": "custom.transform", "value": "negative-Y"}],
                }
            ],
        ),
        # --- 로그 -------------------------------------------------------------
        RowPanel(title="로그", gridPos=GridPos(h=1, w=24, x=0, y=22)),
        Logs(
            title="에러 로그",
            dataSource=LOKI,
            targets=[
                Target(
                    expr='{instance=~"$instance"} |~ "(?i)(error|fail|panic)"',
                    refId="A",
                    datasource=LOKI,
                    intervalFactor=1,
                )
            ],
            gridPos=GridPos(h=10, w=24, x=0, y=23),
            showTime=True,
            wrapLogMessages=True,
            enableLogDetails=True,
            sortOrder="Descending",
            maxDataPoints=NO_MAX_DATA_POINTS,
        ),
    ],
).auto_panel_ids()
