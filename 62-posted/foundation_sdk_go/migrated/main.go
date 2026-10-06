// cmd/convert 가 ../dist/grafanalib.normalized.json 에서 변환한 코드입니다.
// 동작하는 출발점이니, 직접 고쳐가면서 정리하세요.
//
//	go run ./migrated -o ../dist/migrated-go.json
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/grafana/grafana-foundation-sdk/go/cog"
	"github.com/grafana/grafana-foundation-sdk/go/cog/variants"
	"github.com/grafana/grafana-foundation-sdk/go/common"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
	"github.com/grafana/grafana-foundation-sdk/go/logs"
	"github.com/grafana/grafana-foundation-sdk/go/loki"
	"github.com/grafana/grafana-foundation-sdk/go/prometheus"
	"github.com/grafana/grafana-foundation-sdk/go/stat"
	"github.com/grafana/grafana-foundation-sdk/go/timeseries"
)

func dashboardBuilder() *dashboard.DashboardBuilder {
	return dashboard.NewDashboardBuilder("Node Overview").
		Uid("node-overview").
		Title("Node Overview").
		Description("노드 상태 요약 / 리소스 추이 / 에러 로그. grafanalib 과 Foundation SDK 비교용 예제.").
		Tags([]string{"node",
			"homelab",
			"sdk-compare"}).
		Editable().
		Tooltip(1).
		Timepicker(dashboard.NewTimePickerBuilder().
			RefreshIntervals([]string{"5s",
				"10s",
				"30s",
				"1m",
				"5m",
				"15m",
				"30m",
				"1h",
				"2h",
				"1d"})).
		Refresh("30s").
		// [수정 2] 변환기가 시간 범위를 빠뜨려서 직접 추가했습니다.
		Time("now-3h", "now").
		Variables([]cog.Builder[dashboard.VariableModel]{dashboard.NewDatasourceVariableBuilder("datasource").
			Name("datasource").
			Label("Prometheus").
			Hide(0).
			Type("prometheus"),
			dashboard.NewDatasourceVariableBuilder("loki").
				Name("loki").
				Label("Loki").
				Hide(0).
				Type("loki"),
			dashboard.NewQueryVariableBuilder("instance").
				Name("instance").
				Label("Instance").
				Hide(0).
				Query(dashboard.StringOrMap{String: cog.ToPtr[string]("label_values(node_uname_info, instance)")}).
				Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
				Multi(true).
				Refresh(2).
				Sort(1).
				IncludeAll(true)}).
		WithRow(dashboard.NewRowBuilder("요약").
			Title("요약").
			GridPos(dashboard.GridPos{H: 1, W: 24, X: 0, Y: 0}).
			Id(0x1)).
		WithPanel(stat.NewPanelBuilder().
			Id(0x2).
			Targets([]cog.Builder[variants.Dataquery]{prometheus.NewDataqueryBuilder().
				Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
				Expr("sum(up{job=~\".*node.*\", instance=~\"$instance\"})").
				Format("time_series").
				Hide(false).
				RefId("A")}).
			Title("Up").
			Description("수집 중인 node-exporter 수").
			Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
			GridPos(dashboard.GridPos{H: 4, W: 6, X: 0, Y: 1}).
			Height(0x4).
			Span(0x6).
			HideTimeOverride(false).
			Unit("none").
			Decimals(0).
			Thresholds(dashboard.NewThresholdsConfigBuilder().
				Mode("absolute").
				Steps([]dashboard.Threshold{dashboard.Threshold{Color: "red"},
					dashboard.Threshold{Value: cog.ToPtr[float64](1), Color: "green"}})).
			GraphMode("area").
			ColorMode("value").
			JustifyMode("auto").
			TextMode("auto").
			WideLayout(false).
			ReduceOptions(common.NewReduceDataOptionsBuilder().
				Values(false).
				Calcs([]string{"lastNotNull"})).
			PercentChangeColorMode("").
			Orientation("auto")).
		WithPanel(stat.NewPanelBuilder().
			Id(0x3).
			Targets([]cog.Builder[variants.Dataquery]{prometheus.NewDataqueryBuilder().
				Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
				Expr("1 - avg(rate(node_cpu_seconds_total{mode=\"idle\", instance=~\"$instance\"}[$__rate_interval]))").
				Format("time_series").
				Hide(false).
				RefId("A")}).
			Title("CPU 사용률").
			Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
			GridPos(dashboard.GridPos{H: 4, W: 6, X: 6, Y: 1}).
			Height(0x4).
			Span(0x6).
			HideTimeOverride(false).
			Unit("percentunit").
			Decimals(1).
			Min(0).
			Max(1).
			Thresholds(dashboard.NewThresholdsConfigBuilder().
				Mode("absolute").
				Steps([]dashboard.Threshold{dashboard.Threshold{Color: "green"},
					dashboard.Threshold{Value: cog.ToPtr[float64](0.7), Color: "orange"},
					dashboard.Threshold{Value: cog.ToPtr[float64](0.9), Color: "red"}})).
			GraphMode("area").
			ColorMode("value").
			JustifyMode("auto").
			TextMode("auto").
			WideLayout(false).
			ReduceOptions(common.NewReduceDataOptionsBuilder().
				Values(false).
				Calcs([]string{"lastNotNull"})).
			PercentChangeColorMode("").
			Orientation("auto")).
		WithPanel(stat.NewPanelBuilder().
			Id(0x4).
			Targets([]cog.Builder[variants.Dataquery]{prometheus.NewDataqueryBuilder().
				Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
				Expr("1 - sum(node_memory_MemAvailable_bytes{instance=~\"$instance\"}) / sum(node_memory_MemTotal_bytes{instance=~\"$instance\"})").
				Format("time_series").
				Hide(false).
				RefId("A")}).
			Title("Memory 사용률").
			Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
			GridPos(dashboard.GridPos{H: 4, W: 6, X: 12, Y: 1}).
			Height(0x4).
			Span(0x6).
			HideTimeOverride(false).
			Unit("percentunit").
			Decimals(1).
			Min(0).
			Max(1).
			Thresholds(dashboard.NewThresholdsConfigBuilder().
				Mode("absolute").
				Steps([]dashboard.Threshold{dashboard.Threshold{Color: "green"},
					dashboard.Threshold{Value: cog.ToPtr[float64](0.7), Color: "orange"},
					dashboard.Threshold{Value: cog.ToPtr[float64](0.9), Color: "red"}})).
			GraphMode("area").
			ColorMode("value").
			JustifyMode("auto").
			TextMode("auto").
			WideLayout(false).
			ReduceOptions(common.NewReduceDataOptionsBuilder().
				Values(false).
				Calcs([]string{"lastNotNull"})).
			PercentChangeColorMode("").
			Orientation("auto")).
		WithPanel(stat.NewPanelBuilder().
			Id(0x5).
			Targets([]cog.Builder[variants.Dataquery]{prometheus.NewDataqueryBuilder().
				Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
				Expr("1 - sum(node_filesystem_avail_bytes{mountpoint=\"/\", fstype!=\"rootfs\", instance=~\"$instance\"}) / sum(node_filesystem_size_bytes{mountpoint=\"/\", fstype!=\"rootfs\", instance=~\"$instance\"})").
				Format("time_series").
				Hide(false).
				RefId("A")}).
			Title("Root 디스크 사용률").
			Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
			GridPos(dashboard.GridPos{H: 4, W: 6, X: 18, Y: 1}).
			Height(0x4).
			Span(0x6).
			HideTimeOverride(false).
			Unit("percentunit").
			Decimals(1).
			Min(0).
			Max(1).
			Thresholds(dashboard.NewThresholdsConfigBuilder().
				Mode("absolute").
				Steps([]dashboard.Threshold{dashboard.Threshold{Color: "green"},
					dashboard.Threshold{Value: cog.ToPtr[float64](0.8), Color: "orange"},
					dashboard.Threshold{Value: cog.ToPtr[float64](0.9), Color: "red"}})).
			GraphMode("area").
			ColorMode("value").
			JustifyMode("auto").
			TextMode("auto").
			WideLayout(false).
			ReduceOptions(common.NewReduceDataOptionsBuilder().
				Values(false).
				Calcs([]string{"lastNotNull"})).
			PercentChangeColorMode("").
			Orientation("auto")).
		WithRow(dashboard.NewRowBuilder("리소스 추이").
			Title("리소스 추이").
			GridPos(dashboard.GridPos{H: 1, W: 24, X: 0, Y: 5}).
			Id(0x6)).
		WithPanel(timeseries.NewPanelBuilder().
			Id(0x7).
			Targets([]cog.Builder[variants.Dataquery]{prometheus.NewDataqueryBuilder().
				Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
				Expr("1 - avg by(instance) (rate(node_cpu_seconds_total{mode=\"idle\", instance=~\"$instance\"}[$__rate_interval]))").
				Format("time_series").
				Hide(false).
				LegendFormat("{{instance}}").
				RefId("A")}).
			Title("CPU 사용률 (인스턴스별)").
			Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
			GridPos(dashboard.GridPos{H: 8, W: 12, X: 0, Y: 6}).
			Height(0x8).
			HideTimeOverride(false).
			Unit("percentunit").
			Min(0).
			Max(1).
			Thresholds(dashboard.NewThresholdsConfigBuilder().
				Mode("absolute")).
			ColorScheme(dashboard.NewFieldColorBuilder().
				Mode("palette-classic")).
			Legend(common.NewVizLegendOptionsBuilder().
				DisplayMode("table").
				Placement("bottom").
				ShowLegend(false).
				Calcs([]string{"mean",
					"max",
					"lastNotNull"})).
			Tooltip(common.NewVizTooltipOptionsBuilder().
				Mode("multi").
				Sort("desc")).
			DrawStyle("line").
			GradientMode("none").
			ThresholdsStyle(common.NewGraphThresholdsStyleConfigBuilder().
				Mode("off")).
			LineWidth(1).
			LineInterpolation("linear").
			FillOpacity(10).
			ShowPoints("never").
			PointSize(5).
			AxisPlacement("auto").
			ScaleDistribution(common.NewScaleDistributionConfigBuilder().
				Type("linear").
				Log(2)).
			BarAlignment(0).
			HideFrom(common.NewHideSeriesConfigBuilder().
				Tooltip(false).
				Legend(false).
				Viz(false)).
			SpanNulls(common.BoolOrFloat64{Bool: cog.ToPtr[bool](false)})).
		WithPanel(timeseries.NewPanelBuilder().
			Id(0x8).
			Targets([]cog.Builder[variants.Dataquery]{prometheus.NewDataqueryBuilder().
				Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
				Expr("1 - node_memory_MemAvailable_bytes{instance=~\"$instance\"} / node_memory_MemTotal_bytes{instance=~\"$instance\"}").
				Format("time_series").
				Hide(false).
				LegendFormat("{{instance}}").
				RefId("A")}).
			Title("Memory 사용률 (인스턴스별)").
			Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
			GridPos(dashboard.GridPos{H: 8, W: 12, X: 12, Y: 6}).
			Height(0x8).
			HideTimeOverride(false).
			Unit("percentunit").
			Min(0).
			Max(1).
			Thresholds(dashboard.NewThresholdsConfigBuilder().
				Mode("absolute")).
			ColorScheme(dashboard.NewFieldColorBuilder().
				Mode("palette-classic")).
			Legend(common.NewVizLegendOptionsBuilder().
				DisplayMode("table").
				Placement("bottom").
				ShowLegend(false).
				Calcs([]string{"mean",
					"max",
					"lastNotNull"})).
			Tooltip(common.NewVizTooltipOptionsBuilder().
				Mode("multi").
				Sort("desc")).
			DrawStyle("line").
			GradientMode("none").
			ThresholdsStyle(common.NewGraphThresholdsStyleConfigBuilder().
				Mode("off")).
			LineWidth(1).
			LineInterpolation("linear").
			FillOpacity(10).
			ShowPoints("never").
			PointSize(5).
			AxisPlacement("auto").
			ScaleDistribution(common.NewScaleDistributionConfigBuilder().
				Type("linear").
				Log(2)).
			BarAlignment(0).
			HideFrom(common.NewHideSeriesConfigBuilder().
				Tooltip(false).
				Legend(false).
				Viz(false)).
			SpanNulls(common.BoolOrFloat64{Bool: cog.ToPtr[bool](false)})).
		WithPanel(timeseries.NewPanelBuilder().
			Id(0x9).
			Targets([]cog.Builder[variants.Dataquery]{prometheus.NewDataqueryBuilder().
				Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
				Expr("sum by(instance) (rate(node_network_receive_bytes_total{device!~\"lo|veth.*|cali.*|flannel.*|cni.*\", instance=~\"$instance\"}[$__rate_interval]))").
				Format("time_series").
				Hide(false).
				LegendFormat("{{instance}} rx").
				RefId("A"),
				prometheus.NewDataqueryBuilder().
					Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
					Expr("sum by(instance) (rate(node_network_transmit_bytes_total{device!~\"lo|veth.*|cali.*|flannel.*|cni.*\", instance=~\"$instance\"}[$__rate_interval]))").
					Format("time_series").
					Hide(false).
					LegendFormat("{{instance}} tx").
					RefId("B")}).
			Title("Network 송수신").
			Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("prometheus"), Uid: cog.ToPtr[string]("${datasource}")}).
			GridPos(dashboard.GridPos{H: 8, W: 24, X: 0, Y: 14}).
			Height(0x8).
			Span(0x18).
			HideTimeOverride(false).
			Unit("Bps").
			Thresholds(dashboard.NewThresholdsConfigBuilder().
				Mode("absolute")).
			ColorScheme(dashboard.NewFieldColorBuilder().
				Mode("palette-classic")).
			// [수정 1] 변환기가 같은 override 를 WithOverride(...) 로 한 번 더 출력해서, 그 줄을 지웠습니다.
			Overrides([]cog.Builder[dashboard.DashboardFieldConfigSourceOverrides]{dashboard.NewDashboardFieldConfigSourceOverridesBuilder().
				Matcher(dashboard.MatcherConfig{Id: "byRegexp", Options: ".* tx$"}).
				Properties([]dashboard.DynamicConfigValue{dashboard.DynamicConfigValue{Id: "custom.transform", Value: "negative-Y"}})}).
			Legend(common.NewVizLegendOptionsBuilder().
				DisplayMode("table").
				Placement("bottom").
				ShowLegend(false).
				Calcs([]string{"mean",
					"max",
					"lastNotNull"})).
			Tooltip(common.NewVizTooltipOptionsBuilder().
				Mode("multi").
				Sort("desc")).
			DrawStyle("line").
			GradientMode("none").
			ThresholdsStyle(common.NewGraphThresholdsStyleConfigBuilder().
				Mode("off")).
			LineWidth(1).
			LineInterpolation("linear").
			FillOpacity(10).
			ShowPoints("never").
			PointSize(5).
			AxisPlacement("auto").
			ScaleDistribution(common.NewScaleDistributionConfigBuilder().
				Type("linear").
				Log(2)).
			BarAlignment(0).
			HideFrom(common.NewHideSeriesConfigBuilder().
				Tooltip(false).
				Legend(false).
				Viz(false)).
			SpanNulls(common.BoolOrFloat64{Bool: cog.ToPtr[bool](false)})).
		WithRow(dashboard.NewRowBuilder("로그").
			Title("로그").
			GridPos(dashboard.GridPos{H: 1, W: 24, X: 0, Y: 22}).
			Id(0xa)).
		WithPanel(logs.NewPanelBuilder().
			Id(0xb).
			Targets([]cog.Builder[variants.Dataquery]{loki.NewDataqueryBuilder().
				Expr("{instance=~\"$instance\"} |~ \"(?i)(error|fail|panic)\"").
				Instant(false).
				RefId("A").
				Hide(false).
				Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("loki"), Uid: cog.ToPtr[string]("${loki}")})}).
			Title("에러 로그").
			Datasource(common.DataSourceRef{Type: cog.ToPtr[string]("loki"), Uid: cog.ToPtr[string]("${loki}")}).
			GridPos(dashboard.GridPos{H: 10, W: 24, X: 0, Y: 23}).
			Height(0xa).
			Span(0x18).
			HideTimeOverride(false).
			Thresholds(dashboard.NewThresholdsConfigBuilder().
				Mode("absolute")).
			ShowLabels(false).
			ShowCommonLabels(false).
			ShowTime(true).
			ShowLogContextToggle(false).
			WrapLogMessage(true).
			PrettifyLogMessage(false).
			EnableLogDetails(true).
			SortOrder("Descending").
			DedupStrategy("none"))
}

func main() {
	output := flag.String("o", "", "저장할 파일 경로. 없으면 화면에 출력합니다.")
	flag.Parse()

	dash, err := dashboardBuilder().Build()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// 빌더에는 schemaVersion 을 지정하는 메서드가 없어서, 원본 JSON 의 값을 그대로 이어받습니다.
	// (지정하지 않으면 SDK 기본값 42 가 들어갑니다)
	dash.SchemaVersion = 41

	// 키를 정렬해서 출력합니다 (다른 결과물과 diff 하기 쉽게).
	raw, err := json.Marshal(dash)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(generic); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if *output == "" {
		os.Stdout.Write(buf.Bytes())
		return
	}
	if err := os.WriteFile(*output, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println("generated", *output)
}
