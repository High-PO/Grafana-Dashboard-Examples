"""Node Overview — Grafana Foundation SDK (Python) 버전.

grafanalib 버전(../grafanalib/node_overview.dashboard.py)과 같은 대시보드를
Foundation SDK 로 옮긴 코드입니다. Go 버전(../foundation_sdk_go/main.go)과 1:1 로 대응합니다.

    python node_overview.py -o ../dist/fsdk-python.json   # 파일로 저장 (UTF-8)
    python node_overview.py                               # 화면에 출력

명세는 ../SPEC.md 를 참고하세요.
"""

import argparse
import json
import sys

from grafana_foundation_sdk.builders import common as common_builder
from grafana_foundation_sdk.builders import dashboard, logs, loki, prometheus, stat, timeseries
from grafana_foundation_sdk.cog.encoder import JSONEncoder
from grafana_foundation_sdk.models import common
from grafana_foundation_sdk.models import dashboard as dashboard_model
from grafana_foundation_sdk.models.dashboard import Threshold

# ---------------------------------------------------------------------------
# 공통 값
# ---------------------------------------------------------------------------

# SDK(0.0.20) 기본값은 42. 대상 Grafana(12.0.x)가 저장하는 값에 맞춥니다.
SCHEMA_VERSION = 41

PROMETHEUS = common.DataSourceRef(type_val="prometheus", uid="${datasource}")
LOKI = common.DataSourceRef(type_val="loki", uid="${loki}")

INSTANCE = 'instance=~"$instance"'


def prom(expr: str, legend: str = "", ref_id: str = "A") -> prometheus.Dataquery:
    q = prometheus.Dataquery().datasource(PROMETHEUS).expr(expr).ref_id(ref_id)
    if legend:
        q = q.legend_format(legend)
    return q


def thresholds(*steps: Threshold) -> dashboard.ThresholdsConfig:
    return dashboard.ThresholdsConfig().mode(dashboard_model.ThresholdsMode.ABSOLUTE).steps(list(steps))


def base(color: str) -> Threshold:
    """첫 단계(-∞ 부터)."""
    return Threshold(color=color)


def step(value: float, color: str) -> Threshold:
    return Threshold(value=value, color=color)


PERCENT_THRESHOLDS = thresholds(base("green"), step(0.7, "orange"), step(0.9, "red"))
DISK_THRESHOLDS = thresholds(base("green"), step(0.8, "orange"), step(0.9, "red"))


def stat_panel(title: str, expr: str, unit: str = "none") -> stat.Panel:
    """요약 Stat 패널. 한 줄에 4개(span 6) 배치합니다."""
    return (
        stat.Panel()
        .title(title)
        .datasource(PROMETHEUS)
        .with_target(prom(expr))
        .unit(unit)
        .span(6)
        .height(4)
        .color_mode(common.BigValueColorMode.VALUE)
        .graph_mode(common.BigValueGraphMode.AREA)
        # enum 옵션은 명시하자: 지정하지 않으면 Python 은 첫 번째 값("auto"), Go 는 ""(빈 값)을 출력합니다.
        .orientation(common.VizOrientation.AUTO)
        .reduce_options(common_builder.ReduceDataOptions().values(False).calcs(["lastNotNull"]))
        .thresholds(thresholds(base("blue")))
    )


def timeseries_panel(title: str, unit: str) -> timeseries.Panel:
    """추이 그래프. 범례는 표 형태(하단), 툴팁은 전체 시리즈 내림차순."""
    return (
        timeseries.Panel()
        .title(title)
        .datasource(PROMETHEUS)
        .unit(unit)
        .height(8)
        .color_scheme(dashboard.FieldColor().mode(dashboard_model.FieldColorModeId.PALETTE_CLASSIC))
        .draw_style(common.GraphDrawStyle.LINE)
        .line_width(1)
        .fill_opacity(10)
        .show_points(common.VisibilityMode.NEVER)
        .legend(
            common_builder.VizLegendOptions()
            .display_mode(common.LegendDisplayMode.TABLE)
            .placement(common.LegendPlacement.BOTTOM)
            .show_legend(True)
            .calcs(["mean", "max", "lastNotNull"])
        )
        .tooltip(
            common_builder.VizTooltipOptions()
            .mode(common.TooltipDisplayMode.MULTI)
            .sort(common.SortOrder.DESCENDING)
        )
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


def build() -> dashboard_model.Dashboard:
    builder = (
        dashboard.Dashboard("Node Overview")
        .uid("node-overview")
        .description("노드 상태 요약 / 리소스 추이 / 에러 로그. grafanalib 과 Foundation SDK 비교용 예제.")
        .tags(["node", "homelab", "sdk-compare"])
        .timezone("browser")
        .refresh("30s")
        .time("now-3h", "now")
        .tooltip(dashboard_model.DashboardCursorSync.CROSSHAIR)
        # 변수
        .with_variable(dashboard.DatasourceVariable("datasource").label("Prometheus").type("prometheus"))
        .with_variable(dashboard.DatasourceVariable("loki").label("Loki").type("loki"))
        .with_variable(
            dashboard.QueryVariable("instance")
            .label("Instance")
            .datasource(PROMETHEUS)
            .query({"query": "label_values(node_uname_info, instance)", "refId": "StandardVariableQuery"})
            .definition("label_values(node_uname_info, instance)")
            .refresh(dashboard_model.VariableRefresh.ON_TIME_RANGE_CHANGED)
            .sort(dashboard_model.VariableSort.ALPHABETICAL_ASC)
            .multi(True)
            .include_all(True)
        )
        # --- 요약 ---
        .with_row(dashboard.Row("요약"))
        .with_panel(
            stat_panel("Up", f'sum(up{{job=~".*node.*", {INSTANCE}}})')
            .description("수집 중인 node-exporter 수")
            .decimals(0)
            .thresholds(thresholds(base("red"), step(1, "green")))
        )
        .with_panel(stat_panel("CPU 사용률", CPU_USAGE, "percentunit").decimals(1).min(0).max(1).thresholds(PERCENT_THRESHOLDS))
        .with_panel(stat_panel("Memory 사용률", MEM_USAGE, "percentunit").decimals(1).min(0).max(1).thresholds(PERCENT_THRESHOLDS))
        .with_panel(stat_panel("Root 디스크 사용률", DISK_USAGE, "percentunit").decimals(1).min(0).max(1).thresholds(DISK_THRESHOLDS))
        # --- 리소스 추이 ---
        .with_row(dashboard.Row("리소스 추이"))
        .with_panel(
            timeseries_panel("CPU 사용률 (인스턴스별)", "percentunit")
            .span(12)
            .min(0)
            .max(1)
            .with_target(prom(CPU_USAGE_BY_INSTANCE, "{{instance}}"))
        )
        .with_panel(
            timeseries_panel("Memory 사용률 (인스턴스별)", "percentunit")
            .span(12)
            .min(0)
            .max(1)
            .with_target(prom(MEM_USAGE_BY_INSTANCE, "{{instance}}"))
        )
        .with_panel(
            timeseries_panel("Network 송수신", "Bps")
            .span(24)
            .with_target(
                prom(
                    f"sum by(instance) (rate(node_network_receive_bytes_total{{{NET_DEVICE}}}[$__rate_interval]))",
                    "{{instance}} rx",
                    "A",
                )
            )
            .with_target(
                prom(
                    f"sum by(instance) (rate(node_network_transmit_bytes_total{{{NET_DEVICE}}}[$__rate_interval]))",
                    "{{instance}} tx",
                    "B",
                )
            )
            # 송신(tx)은 아래(-)로 그립니다.
            .override_by_regexp(
                ".* tx$",
                [dashboard_model.DynamicConfigValue(id_val="custom.transform", value="negative-Y")],
            )
        )
        # --- 로그 ---
        .with_row(dashboard.Row("로그"))
        .with_panel(
            logs.Panel()
            .title("에러 로그")
            .datasource(LOKI)
            .span(24)
            .height(10)
            .with_target(
                loki.Dataquery()
                .datasource(LOKI)
                .expr('{instance=~"$instance"} |~ "(?i)(error|fail|panic)"')
                .ref_id("A")
            )
            .show_time(True)
            .wrap_log_message(True)
            .enable_log_details(True)
            .sort_order(common.LogsSortOrder.DESCENDING)
            .dedup_strategy(common.LogsDedupStrategy.NONE)
        )
    )

    dash = builder.build()

    # 후처리: SDK 는 패널 ID 를 붙이지 않으므로 위에서부터 순서대로 부여합니다.
    next_id = 1
    for item in dash.panels or []:
        item.id_val = next_id
        next_id += 1
        for child in getattr(item, "panels", None) or []:
            child.id_val = next_id
            next_id += 1

    dash.schema_version = SCHEMA_VERSION
    return dash


def to_json(dash: dashboard_model.Dashboard) -> str:
    return json.dumps(dash, cls=JSONEncoder, indent=2, ensure_ascii=False, sort_keys=True)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("-o", "--output", help="저장할 파일 경로. 없으면 화면에 출력합니다.")
    args = parser.parse_args()

    output = to_json(build()) + "\n"
    if args.output:
        # Windows PowerShell 의 > 리다이렉트는 인코딩 문제(UTF-16, 한글 깨짐)가 있어서 파일로 직접 씁니다.
        with open(args.output, "w", encoding="utf-8", newline="\n") as f:
            f.write(output)
        print(f"generated {args.output}")
    else:
        sys.stdout.reconfigure(encoding="utf-8")
        sys.stdout.write(output)
