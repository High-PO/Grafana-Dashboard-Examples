// Node Overview — Grafana Foundation SDK (Go) 버전.
//
// Python 버전(../foundation_sdk_python/node_overview.py)과 1:1 로 대응합니다.
// 같은 SDK 라서 빌더 이름만 CamelCase ↔ snake_case 로 바뀌고, 생성되는 JSON 은 완전히 같습니다.
//
//	go run . -o ../dist/fsdk-go.json   // 파일로 저장 (UTF-8)
//	go run .                           // 화면에 출력
//
// 명세는 ../SPEC.md 를 참고하세요.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/grafana/grafana-foundation-sdk/go/cog"
	"github.com/grafana/grafana-foundation-sdk/go/common"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
	"github.com/grafana/grafana-foundation-sdk/go/logs"
	"github.com/grafana/grafana-foundation-sdk/go/loki"
	"github.com/grafana/grafana-foundation-sdk/go/prometheus"
	"github.com/grafana/grafana-foundation-sdk/go/stat"
	"github.com/grafana/grafana-foundation-sdk/go/timeseries"
)

// ---------------------------------------------------------------------------
// 공통 값
// ---------------------------------------------------------------------------

// SDK(v0.0.20) 기본값은 42. 대상 Grafana(12.0.x)가 저장하는 값에 맞춥니다.
const schemaVersion = 41

var (
	promDS = common.DataSourceRef{Type: cog.ToPtr("prometheus"), Uid: cog.ToPtr("${datasource}")}
	lokiDS = common.DataSourceRef{Type: cog.ToPtr("loki"), Uid: cog.ToPtr("${loki}")}
)

const instance = `instance=~"$instance"`

func prom(expr, legend, refID string) *prometheus.DataqueryBuilder {
	q := prometheus.NewDataqueryBuilder().Datasource(promDS).Expr(expr).RefId(refID)
	if legend != "" {
		q = q.LegendFormat(legend)
	}
	return q
}

func thresholds(steps ...dashboard.Threshold) *dashboard.ThresholdsConfigBuilder {
	return dashboard.NewThresholdsConfigBuilder().Mode(dashboard.ThresholdsModeAbsolute).Steps(steps)
}

// base 는 첫 단계(-∞ 부터)입니다.
func base(color string) dashboard.Threshold { return dashboard.Threshold{Color: color} }

func step(value float64, color string) dashboard.Threshold {
	return dashboard.Threshold{Value: cog.ToPtr(value), Color: color}
}

var (
	percentThresholds = thresholds(base("green"), step(0.7, "orange"), step(0.9, "red"))
	diskThresholds    = thresholds(base("green"), step(0.8, "orange"), step(0.9, "red"))
)

// statPanel 은 요약 Stat 패널입니다. 한 줄에 4개(span 6) 배치합니다.
func statPanel(title, expr, unit string) *stat.PanelBuilder {
	return stat.NewPanelBuilder().
		Title(title).
		Datasource(promDS).
		WithTarget(prom(expr, "", "A")).
		Unit(unit).
		Span(6).
		Height(4).
		ColorMode(common.BigValueColorModeValue).
		GraphMode(common.BigValueGraphModeArea).
		// enum 옵션은 명시하자: 지정하지 않으면 Python 은 첫 번째 값("auto"), Go 는 ""(빈 값)을 출력합니다.
		Orientation(common.VizOrientationAuto).
		ReduceOptions(common.NewReduceDataOptionsBuilder().Values(false).Calcs([]string{"lastNotNull"})).
		Thresholds(thresholds(base("blue")))
}

// timeseriesPanel 은 추이 그래프입니다. 범례는 표 형태(하단), 툴팁은 전체 시리즈 내림차순.
func timeseriesPanel(title, unit string) *timeseries.PanelBuilder {
	return timeseries.NewPanelBuilder().
		Title(title).
		Datasource(promDS).
		Unit(unit).
		Height(8).
		ColorScheme(dashboard.NewFieldColorBuilder().Mode(dashboard.FieldColorModeIdPaletteClassic)).
		DrawStyle(common.GraphDrawStyleLine).
		LineWidth(1).
		FillOpacity(10).
		ShowPoints(common.VisibilityModeNever).
		Legend(common.NewVizLegendOptionsBuilder().
			DisplayMode(common.LegendDisplayModeTable).
			Placement(common.LegendPlacementBottom).
			ShowLegend(true).
			Calcs([]string{"mean", "max", "lastNotNull"})).
		Tooltip(common.NewVizTooltipOptionsBuilder().
			Mode(common.TooltipDisplayModeMulti).
			Sort(common.SortOrderDescending))
}

// ---------------------------------------------------------------------------
// PromQL
// ---------------------------------------------------------------------------

var (
	cpuUsage           = `1 - avg(rate(node_cpu_seconds_total{mode="idle", ` + instance + `}[$__rate_interval]))`
	cpuUsageByInstance = `1 - avg by(instance) (rate(node_cpu_seconds_total{mode="idle", ` + instance + `}[$__rate_interval]))`
	memUsage           = `1 - sum(node_memory_MemAvailable_bytes{` + instance + `}) / sum(node_memory_MemTotal_bytes{` + instance + `})`
	memUsageByInstance = `1 - node_memory_MemAvailable_bytes{` + instance + `} / node_memory_MemTotal_bytes{` + instance + `}`
	rootFS             = `mountpoint="/", fstype!="rootfs", ` + instance
	diskUsage          = `1 - sum(node_filesystem_avail_bytes{` + rootFS + `}) / sum(node_filesystem_size_bytes{` + rootFS + `})`
	netDevice          = `device!~"lo|veth.*|cali.*|flannel.*|cni.*", ` + instance
)

// ---------------------------------------------------------------------------
// 대시보드
// ---------------------------------------------------------------------------

func build() (dashboard.Dashboard, error) {
	builder := dashboard.NewDashboardBuilder("Node Overview").
		Uid("node-overview").
		Description("노드 상태 요약 / 리소스 추이 / 에러 로그. grafanalib 과 Foundation SDK 비교용 예제.").
		Tags([]string{"node", "homelab", "sdk-compare"}).
		Timezone("browser").
		Refresh("30s").
		Time("now-3h", "now").
		Tooltip(dashboard.DashboardCursorSyncCrosshair).
		// 변수
		WithVariable(dashboard.NewDatasourceVariableBuilder("datasource").Label("Prometheus").Type("prometheus")).
		WithVariable(dashboard.NewDatasourceVariableBuilder("loki").Label("Loki").Type("loki")).
		WithVariable(dashboard.NewQueryVariableBuilder("instance").
			Label("Instance").
			Datasource(promDS).
			Query(dashboard.StringOrMap{Map: map[string]any{
				"query": "label_values(node_uname_info, instance)",
				"refId": "StandardVariableQuery",
			}}).
			Definition("label_values(node_uname_info, instance)").
			Refresh(dashboard.VariableRefreshOnTimeRangeChanged).
			Sort(dashboard.VariableSortAlphabeticalAsc).
			Multi(true).
			IncludeAll(true)).
		// --- 요약 ---
		WithRow(dashboard.NewRowBuilder("요약")).
		WithPanel(statPanel("Up", `sum(up{job=~".*node.*", `+instance+`})`, "none").
			Description("수집 중인 node-exporter 수").
			Decimals(0).
			Thresholds(thresholds(base("red"), step(1, "green")))).
		WithPanel(statPanel("CPU 사용률", cpuUsage, "percentunit").Decimals(1).Min(0).Max(1).Thresholds(percentThresholds)).
		WithPanel(statPanel("Memory 사용률", memUsage, "percentunit").Decimals(1).Min(0).Max(1).Thresholds(percentThresholds)).
		WithPanel(statPanel("Root 디스크 사용률", diskUsage, "percentunit").Decimals(1).Min(0).Max(1).Thresholds(diskThresholds)).
		// --- 리소스 추이 ---
		WithRow(dashboard.NewRowBuilder("리소스 추이")).
		WithPanel(timeseriesPanel("CPU 사용률 (인스턴스별)", "percentunit").
			Span(12).Min(0).Max(1).
			WithTarget(prom(cpuUsageByInstance, "{{instance}}", "A"))).
		WithPanel(timeseriesPanel("Memory 사용률 (인스턴스별)", "percentunit").
			Span(12).Min(0).Max(1).
			WithTarget(prom(memUsageByInstance, "{{instance}}", "A"))).
		WithPanel(timeseriesPanel("Network 송수신", "Bps").
			Span(24).
			WithTarget(prom(`sum by(instance) (rate(node_network_receive_bytes_total{`+netDevice+`}[$__rate_interval]))`, "{{instance}} rx", "A")).
			WithTarget(prom(`sum by(instance) (rate(node_network_transmit_bytes_total{`+netDevice+`}[$__rate_interval]))`, "{{instance}} tx", "B")).
			// 송신(tx)은 아래(-)로 그립니다.
			OverrideByRegexp(".* tx$", []dashboard.DynamicConfigValue{
				{Id: "custom.transform", Value: "negative-Y"},
			})).
		// --- 로그 ---
		WithRow(dashboard.NewRowBuilder("로그")).
		WithPanel(logs.NewPanelBuilder().
			Title("에러 로그").
			Datasource(lokiDS).
			Span(24).
			Height(10).
			WithTarget(loki.NewDataqueryBuilder().
				Datasource(lokiDS).
				Expr(`{instance=~"$instance"} |~ "(?i)(error|fail|panic)"`).
				RefId("A")).
			ShowTime(true).
			WrapLogMessage(true).
			EnableLogDetails(true).
			SortOrder(common.LogsSortOrderDescending).
			DedupStrategy(common.LogsDedupStrategyNone))

	dash, err := builder.Build()
	if err != nil {
		return dashboard.Dashboard{}, err
	}

	// 후처리: SDK 는 패널 ID 를 붙이지 않으므로 위에서부터 순서대로 부여합니다.
	var nextID uint32 = 1
	for _, item := range dash.Panels {
		switch {
		case item.Panel != nil:
			item.Panel.Id = cog.ToPtr(nextID)
			nextID++
		case item.RowPanel != nil:
			item.RowPanel.Id = nextID
			nextID++
			for i := range item.RowPanel.Panels {
				item.RowPanel.Panels[i].Id = cog.ToPtr(nextID)
				nextID++
			}
		}
	}

	dash.SchemaVersion = schemaVersion
	return dash, nil
}

func main() {
	output := flag.String("o", "", "저장할 파일 경로. 없으면 화면에 출력합니다.")
	flag.Parse()

	dash, err := build()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	out, err := marshalSorted(dash)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if *output == "" {
		os.Stdout.Write(out)
		return
	}
	// Windows PowerShell 의 > 리다이렉트는 인코딩 문제(UTF-16, 한글 깨짐)가 있어서 파일로 직접 씁니다.
	if err := os.WriteFile(*output, out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println("generated", *output)
}

// marshalSorted 는 키를 알파벳 순으로 정렬한 JSON 을 만듭니다.
// Python 버전(json.dumps(sort_keys=True))과 같은 모양이 되어서, 두 파일을 그대로 diff 할 수 있습니다.
func marshalSorted(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any // map 으로 다시 읽으면 encoding/json 이 키를 정렬해서 출력합니다.
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(generic); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
