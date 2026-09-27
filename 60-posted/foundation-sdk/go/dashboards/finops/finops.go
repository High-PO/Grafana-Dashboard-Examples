// Package finops 는 59-posted 의 "FinOps — AWS 환산 비용 (OpenCost)" 대시보드를
// Foundation SDK 코드로 옮긴(migration) 예제입니다.
//
// 원본 JSON: 59-posted/FinOps — AWS 환산 비용 (OpenCost).json
//
// 옮기는 과정:
//  1. tools/convert 로 원본 JSON 을 Go 코드 초안으로 변환 (go run ./tools/convert <json>)
//  2. 초안에서 반복되는 부분(데이터소스, Stat 기본 옵션, 비용 PromQL)을 헬퍼/상수로 정리
//  3. gridPos 는 지우고 Span/Height 만 남겨 SDK 의 자동 배치에 맡김
//  4. tools/compare 로 원본과 생성 결과의 의미 있는 필드(쿼리/단위/임계값/배치)가 같은지 확인
package finops

import (
	"fmt"

	"github.com/grafana/grafana-foundation-sdk/go/barchart"
	"github.com/grafana/grafana-foundation-sdk/go/cog"
	sdkcommon "github.com/grafana/grafana-foundation-sdk/go/common"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
	"github.com/grafana/grafana-foundation-sdk/go/piechart"
	"github.com/grafana/grafana-foundation-sdk/go/stat"
	"github.com/grafana/grafana-foundation-sdk/go/table"
	"github.com/grafana/grafana-foundation-sdk/go/timeseries"

	"github.com/High-PO/Grafana-Dashboard-Examples/60-posted/foundation-sdk/go/internal/common"
)

const (
	// UID 는 원본과 동일하게 유지합니다. 이미 홈랩에 있는 대시보드를 Terraform 으로 가져오려면(import)
	// UID 가 같아야 합니다. (README 의 "기존 대시보드 가져오기" 참고)
	UID   = "opencost-homelab"
	Title = "FinOps — AWS 환산 비용 (OpenCost)"
)

// 1달 = 730시간, 1년 = 8760시간 (AWS 요금 계산과 동일한 기준)
const (
	hoursPerMonth = 730
	hoursPerYear  = 8760
)

// PromQL 조각. 원본 JSON 에서는 같은 식이 패널마다 복붙되어 있었는데, 코드에서는 한 번만 정의합니다.
const (
	cpuCostByNamespace = `sum by(namespace) (container_cpu_allocation{namespace=~"$namespace"} * on(node) group_left() avg by(node) (node_cpu_hourly_cost))`
	memCostByNamespace = `sum by(namespace) (container_memory_allocation_bytes{namespace=~"$namespace"} / 1024 / 1024 / 1024 * on(node) group_left() avg by(node) (node_ram_hourly_cost))`
)

var (
	monthlyCostByNamespace = fmt.Sprintf("%s * %d\n+\n%s * %d", cpuCostByNamespace, hoursPerMonth, memCostByNamespace, hoursPerMonth)
	efficiencyThresholds   = common.Thresholds(common.Base("red"), common.Step(0.3, "orange"), common.Step(0.5, "green"))
)

// Build 는 대시보드를 생성합니다.
func Build() (dashboard.Dashboard, error) {
	builder := dashboard.NewDashboardBuilder(Title).
		Uid(UID).
		Description("OpenCost 커스텀 가격 모델(AWS 온디맨드 환산) 기반 홈랩 비용 대시보드").
		Tags([]string{"cost", "opencost", "finops"}).
		Editable().
		Tooltip(dashboard.DashboardCursorSyncCrosshair).
		Refresh("1m").
		Time("now-24h", "now").
		Preload(false).
		Annotation(common.BuiltInAnnotation()).
		// 변수
		WithVariable(common.DatasourceVariable().
			Current(dashboard.VariableOption{
				Text:  dashboard.StringOrArrayOfString{String: cog.ToPtr("Prometheus")},
				Value: dashboard.StringOrArrayOfString{String: cog.ToPtr("prometheus")},
			})).
		WithVariable(common.QueryVariable("namespace", "Namespace",
			`label_values(container_cpu_allocation, namespace)`).
			Multi(true).
			IncludeAll(true).
			AllValue(".*").
			Current(dashboard.VariableOption{
				Text:  dashboard.StringOrArrayOfString{String: cog.ToPtr("All")},
				Value: dashboard.StringOrArrayOfString{ArrayOfString: []string{"$__all"}},
			})).
		WithVariable(common.CustomVariable("krw", "환율(원/USD)",
			[]string{"1300", "1380", "1400", "1450"}, "1380").
			Description("원/달러 환율. 원화 환산 패널에 사용됩니다.")).
		WithVariable(common.CustomVariable("homelab_cost", "홈랩 월 비용(원)",
			[]string{"15000", "29000", "50000", "80000"}, "29000").
			Description("홈랩 월 실제 비용(전기요금 등, 원). 절감액 계산에 사용됩니다.")).
		// 행: AWS였다면 얼마인가
		WithRow(dashboard.NewRowBuilder("AWS였다면 얼마인가")).
		WithPanel(costStat("시간당", "시간당", "sum(node_total_hourly_cost)", 2, "blue").
			Description("노드 전체를 AWS 온디맨드로 돌렸을 때의 시간당 비용")).
		WithPanel(costStat("월 환산", "월", fmt.Sprintf("sum(node_total_hourly_cost) * %d", hoursPerMonth), 0, "blue")).
		WithPanel(costStat("연 환산", "연", fmt.Sprintf("sum(node_total_hourly_cost) * %d", hoursPerYear), 0, "purple").
			GraphMode(sdkcommon.BigValueGraphModeNone)).
		WithPanel(savings()).
		WithPanel(cpuEfficiency()).
		// 행: 네임스페이스별
		WithRow(dashboard.NewRowBuilder("네임스페이스별")).
		WithPanel(costByNamespace()).
		WithPanel(costShare()).
		WithPanel(costDetailTable()).
		// 행: 노드 / 스토리지
		WithRow(dashboard.NewRowBuilder("노드 / 스토리지")).
		WithPanel(costByNode()).
		WithPanel(pvCostTop())

	return common.Build(builder)
}

// ---------------------------------------------------------------------------
// 행 1: 요약 Stat
// ---------------------------------------------------------------------------

// costStat 은 "AWS였다면 얼마인가" 행의 비용 Stat 패널입니다.
func costStat(title, legend, expr string, decimals float64, color string) *stat.PanelBuilder {
	return common.Stat(title, "currencyUSD").
		Span(4).
		Height(4).
		Decimals(decimals).
		Thresholds(common.Thresholds(common.Base(color))).
		PercentChangeColorMode(sdkcommon.PercentChangeColorModeStandard).
		Orientation(sdkcommon.VizOrientationAuto).
		WideLayout(true).
		WithTarget(common.Query(expr).LegendFormat(legend).RefId("A"))
}

func savings() *stat.PanelBuilder {
	return common.Stat("월 절감액 (원)", "currencyKRW").
		Description("AWS 월 환산액을 원화로 바꾼 뒤, 홈랩 실제 월 비용(변수)을 뺀 값").
		Span(6).
		Height(4).
		Decimals(0).
		Thresholds(common.Thresholds(common.Base("green"))).
		PercentChangeColorMode(sdkcommon.PercentChangeColorModeStandard).
		Orientation(sdkcommon.VizOrientationAuto).
		WideLayout(true).
		WithTarget(common.Query(fmt.Sprintf("sum(node_total_hourly_cost) * %d * $krw - $homelab_cost", hoursPerMonth)).
			LegendFormat("절감").
			RefId("A"))
}

func cpuEfficiency() *stat.PanelBuilder {
	return common.Stat("CPU 효율", "percentunit").
		Description("할당된 리소스 중 실제로 쓰이는 비율. 낮을수록 request 과다 설정.").
		Span(6).
		Height(4).
		Decimals(1).
		Min(0).
		Max(1).
		Thresholds(efficiencyThresholds).
		PercentChangeColorMode(sdkcommon.PercentChangeColorModeStandard).
		Orientation(sdkcommon.VizOrientationAuto).
		WideLayout(true).
		WithTarget(common.Query(`sum(rate(container_cpu_usage_seconds_total{container!="", container!="POD"}[10m])) / sum(container_cpu_allocation)`).
			LegendFormat("CPU 효율").
			RefId("A"))
}

// ---------------------------------------------------------------------------
// 행 2: 네임스페이스별
// ---------------------------------------------------------------------------

func costByNamespace() *timeseries.PanelBuilder {
	return common.Timeseries("네임스페이스별 월 환산 비용", "currencyUSD").
		Description("각 네임스페이스가 점유한 CPU/메모리를 AWS 단가로 환산한 월 비용").
		Span(14).
		Height(10).
		Min(0).
		Thresholds(common.Thresholds(common.Base("green"))).
		FillOpacity(25).
		GradientMode(sdkcommon.GraphGradientModeOpacity).
		Stacking(sdkcommon.NewStackingConfigBuilder().
			Mode(sdkcommon.StackingModeNormal).
			Group("A")).
		Legend(sdkcommon.NewVizLegendOptionsBuilder().
			DisplayMode(sdkcommon.LegendDisplayModeTable).
			Placement(sdkcommon.LegendPlacementRight).
			ShowLegend(true).
			SortBy("Last *").
			SortDesc(true).
			Calcs([]string{"mean", "lastNotNull"})).
		WithTarget(common.Query(monthlyCostByNamespace).LegendFormat("{{namespace}}").RefId("A"))
}

func costShare() *piechart.PanelBuilder {
	return piechart.NewPanelBuilder().
		Title("비용 비중").
		Datasource(common.Datasource()).
		Span(10).
		Height(10).
		Unit("currencyUSD").
		ColorScheme(common.ColorMode(dashboard.FieldColorModeIdPaletteClassic)).
		PieType(piechart.PieChartTypeDonut).
		DisplayLabels([]piechart.PieChartLabels{piechart.PieChartLabelsPercent}).
		ReduceOptions(sdkcommon.NewReduceDataOptionsBuilder().
			Values(false).
			Calcs([]string{"lastNotNull"})).
		Legend(piechart.NewPieChartLegendOptionsBuilder().
			DisplayMode(sdkcommon.LegendDisplayModeTable).
			Placement(sdkcommon.LegendPlacementRight).
			ShowLegend(true).
			Values([]piechart.PieChartLegendValues{piechart.PieChartLegendValuesValue})).
		Tooltip(sdkcommon.NewVizTooltipOptionsBuilder().
			Mode(sdkcommon.TooltipDisplayModeSingle).
			Sort(sdkcommon.SortOrderDescending)).
		WithTarget(common.Query(monthlyCostByNamespace).LegendFormat("{{namespace}}").RefId("A"))
}

func costDetailTable() *table.PanelBuilder {
	return table.NewPanelBuilder().
		Title("네임스페이스별 비용 상세 (월 환산)").
		Description("CPU/메모리 비용과 실제 사용 효율을 함께 봅니다. 효율이 낮은데 비용이 큰 항목이 최적화 1순위입니다.").
		Datasource(common.Datasource()).
		Span(24).
		Height(11).
		Unit("currencyUSD").
		ColorScheme(common.ColorMode(dashboard.FieldColorModeIdThresholds)).
		Thresholds(common.Thresholds(common.Base("text"))).
		Filterable(true).
		CellHeight(sdkcommon.TableCellHeightSm).
		Footer(sdkcommon.NewTableFooterOptionsBuilder().
			Show(true).
			Reducer([]string{"sum"}).
			CountRows(false)).
		SortBy([]cog.Builder[sdkcommon.TableSortByFieldState]{
			sdkcommon.NewTableSortByFieldStateBuilder().DisplayName("월 합계").Desc(true),
		}).
		WithTarget(common.TableQuery(fmt.Sprintf("%s * %d", cpuCostByNamespace, hoursPerMonth)).RefId("A")).
		WithTarget(common.TableQuery(fmt.Sprintf("%s * %d", memCostByNamespace, hoursPerMonth)).RefId("B")).
		WithTarget(common.TableQuery(`sum by(namespace) (rate(container_cpu_usage_seconds_total{container!="", container!="POD", namespace=~"$namespace"}[10m])) / sum by(namespace) (container_cpu_allocation{namespace=~"$namespace"})`).RefId("C")).
		WithTarget(common.TableQuery(fmt.Sprintf("(%s\n+\n%s) * %d", cpuCostByNamespace, memCostByNamespace, hoursPerMonth)).RefId("D")).
		// 4개 쿼리를 namespace 기준으로 합치고, 컬럼 이름을 사람이 읽기 좋게 바꿉니다.
		WithTransformation(dashboard.DataTransformerConfig{
			Id:      "joinByField",
			Options: map[string]any{"byField": "namespace", "mode": "outer"},
		}).
		WithTransformation(dashboard.DataTransformerConfig{
			Id: "organize",
			Options: map[string]any{
				"excludeByName": map[string]any{"Time": true, "Time 1": true, "Time 2": true, "Time 3": true, "Time 4": true},
				"indexByName":   map[string]any{},
				"renameByName": map[string]any{
					"Value #A":  "CPU 비용",
					"Value #B":  "메모리 비용",
					"Value #C":  "CPU 효율",
					"Value #D":  "월 합계",
					"namespace": "Namespace",
				},
			},
		}).
		OverrideByName("CPU 효율", []dashboard.DynamicConfigValue{
			{Id: "unit", Value: "percentunit"},
			{Id: "decimals", Value: 1},
			{Id: "custom.cellOptions", Value: map[string]any{"type": "color-background", "mode": "gradient"}},
			{Id: "thresholds", Value: mustBuild(efficiencyThresholds)},
		}).
		OverrideByName("월 합계", []dashboard.DynamicConfigValue{
			{Id: "custom.cellOptions", Value: map[string]any{"type": "gauge", "mode": "lcd"}},
		})
}

// ---------------------------------------------------------------------------
// 행 3: 노드 / 스토리지
// ---------------------------------------------------------------------------

func costByNode() *timeseries.PanelBuilder {
	return common.Timeseries("노드별 월 환산 비용", "currencyUSD").
		Description("커스텀 가격 모델이 제대로 적용됐는지 확인하는 용도이기도 합니다.").
		Span(12).
		Height(9).
		Min(0).
		LineWidth(2).
		FillOpacity(0).
		Thresholds(common.Thresholds(common.Base("green"))).
		Legend(sdkcommon.NewVizLegendOptionsBuilder().
			DisplayMode(sdkcommon.LegendDisplayModeTable).
			Placement(sdkcommon.LegendPlacementBottom).
			ShowLegend(true).
			SortBy("Last *").
			SortDesc(true).
			Calcs([]string{"lastNotNull"})).
		WithTarget(common.Query(fmt.Sprintf("node_total_hourly_cost * %d", hoursPerMonth)).LegendFormat("{{node}}").RefId("A"))
}

func pvCostTop() *barchart.PanelBuilder {
	return barchart.NewPanelBuilder().
		Title("PV 월 환산 비용 Top 15").
		Description("PV 용량 × gp3 단가. 안 쓰는 볼륨이 남아 있으면 여기서 드러납니다.").
		Datasource(common.Datasource()).
		Span(12).
		Height(9).
		Unit("currencyUSD").
		ColorScheme(common.ColorMode(dashboard.FieldColorModeIdPaletteClassic)).
		Thresholds(common.Thresholds(common.Base("green"))).
		Orientation(sdkcommon.VizOrientationHorizontal).
		ShowValue(sdkcommon.VisibilityModeAuto).
		Stacking(sdkcommon.StackingModeNone).
		BarWidth(0.7).
		GroupWidth(0.7).
		FillOpacity(80).
		Legend(sdkcommon.NewVizLegendOptionsBuilder().
			DisplayMode(sdkcommon.LegendDisplayModeList).
			Placement(sdkcommon.LegendPlacementBottom).
			ShowLegend(false)).
		Tooltip(sdkcommon.NewVizTooltipOptionsBuilder().
			Mode(sdkcommon.TooltipDisplayModeSingle).
			Sort(sdkcommon.SortOrderNone)).
		WithTarget(common.TableQuery(`topk(15, sum by(persistentvolume) (kube_persistentvolume_capacity_bytes / 1024 / 1024 / 1024) * 0.0912)`).
			LegendFormat("{{persistentvolume}}").
			RefId("A"))
}

// mustBuild 는 override 값처럼 "빌더가 아닌 값"이 필요한 곳에서 빌더를 즉시 빌드합니다.
func mustBuild[T any](b cog.Builder[T]) T {
	v, err := b.Build()
	if err != nil {
		panic(err)
	}
	return v
}
