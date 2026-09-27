// Package common 은 여러 대시보드에서 반복되는 설정(데이터소스, 변수, 패널 기본값)을 모아둔 헬퍼입니다.
//
// Foundation SDK 빌더는 옵션이 매우 많기 때문에, 대시보드마다 같은 옵션을 반복해서 적으면
// JSON 을 직접 관리할 때와 다를 바가 없습니다. "우리 팀 기본값"은 여기에서 한 번만 정의합니다.
package common

import (
	"strings"

	"github.com/grafana/grafana-foundation-sdk/go/cog"
	sdkcommon "github.com/grafana/grafana-foundation-sdk/go/common"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
	"github.com/grafana/grafana-foundation-sdk/go/prometheus"
	"github.com/grafana/grafana-foundation-sdk/go/stat"
	"github.com/grafana/grafana-foundation-sdk/go/timeseries"
)

// SchemaVersion 은 생성되는 대시보드 JSON 의 schemaVersion 입니다.
//
// SDK(v0.0.20)의 기본값은 42 이지만, 홈랩 Grafana(12.0.x)가 저장하는 값은 41 입니다.
// 대상 Grafana 가 아는 버전으로 맞춰두면 불필요한 마이그레이션/diff 를 피할 수 있습니다.
// Grafana 를 업그레이드하면 이 값도 함께 올려주세요.
const SchemaVersion = 41

// DatasourceVarName 은 모든 대시보드가 공통으로 쓰는 Prometheus 데이터소스 변수 이름입니다.
const DatasourceVarName = "datasource"

// Datasource 는 패널/쿼리에서 참조하는 데이터소스입니다. 특정 UID 를 박아넣지 않고
// ${datasource} 변수를 참조하므로, 환경(로컬/홈랩)마다 데이터소스 UID 가 달라도 그대로 동작합니다.
func Datasource() sdkcommon.DataSourceRef {
	return sdkcommon.DataSourceRef{
		Type: cog.ToPtr("prometheus"),
		Uid:  cog.ToPtr("${" + DatasourceVarName + "}"),
	}
}

// ---------------------------------------------------------------------------
// 변수(Variables)
// ---------------------------------------------------------------------------

// DatasourceVariable 은 Prometheus 타입 데이터소스를 고르는 변수입니다.
func DatasourceVariable() *dashboard.DatasourceVariableBuilder {
	return dashboard.NewDatasourceVariableBuilder(DatasourceVarName).
		Label("Datasource").
		Type("prometheus")
}

// QueryVariable 은 label_values(...) 같은 PromQL 결과로 값을 채우는 변수입니다.
// 시간 범위가 바뀔 때마다 다시 조회하고, 알파벳 순으로 정렬합니다.
func QueryVariable(name, label, query string) *dashboard.QueryVariableBuilder {
	return dashboard.NewQueryVariableBuilder(name).
		Label(label).
		Datasource(Datasource()).
		Query(dashboard.StringOrMap{Map: map[string]any{
			"query": query,
			"refId": "StandardVariableQuery",
		}}).
		Definition(query).
		Refresh(dashboard.VariableRefreshOnTimeRangeChanged).
		Sort(dashboard.VariableSortAlphabeticalAsc)
}

// CustomVariable 은 고정된 선택지 목록을 가진 변수입니다. current 는 기본 선택값입니다.
func CustomVariable(name, label string, values []string, current string) *dashboard.CustomVariableBuilder {
	options := make([]dashboard.VariableOption, 0, len(values))
	for _, v := range values {
		options = append(options, dashboard.VariableOption{
			Selected: cog.ToPtr(v == current),
			Text:     dashboard.StringOrArrayOfString{String: cog.ToPtr(v)},
			Value:    dashboard.StringOrArrayOfString{String: cog.ToPtr(v)},
		})
	}

	return dashboard.NewCustomVariableBuilder(name).
		Label(label).
		Values(dashboard.StringOrMap{String: cog.ToPtr(strings.Join(values, ","))}).
		Current(dashboard.VariableOption{
			Text:  dashboard.StringOrArrayOfString{String: cog.ToPtr(current)},
			Value: dashboard.StringOrArrayOfString{String: cog.ToPtr(current)},
		}).
		Options(options)
}

// BuiltInAnnotation 은 Grafana 기본 "Annotations & Alerts" 주석입니다.
// UI 에서 대시보드를 만들면 자동으로 들어가는 항목이라, 코드로 만들 때도 동일하게 넣어줍니다.
func BuiltInAnnotation() *dashboard.AnnotationQueryBuilder {
	return dashboard.NewAnnotationQueryBuilder().
		Name("Annotations & Alerts").
		Datasource(sdkcommon.DataSourceRef{Type: cog.ToPtr("grafana"), Uid: cog.ToPtr("-- Grafana --")}).
		Enable(true).
		Hide(true).
		IconColor("rgba(0, 211, 255, 1)").
		Type("dashboard").
		BuiltIn(1)
}

// ---------------------------------------------------------------------------
// 쿼리 / 임계값
// ---------------------------------------------------------------------------

// Query 는 ${datasource} 를 바라보는 PromQL 쿼리입니다.
func Query(expr string) *prometheus.DataqueryBuilder {
	return prometheus.NewDataqueryBuilder().
		Datasource(Datasource()).
		Expr(expr)
}

// TableQuery 는 테이블 패널용 instant 쿼리입니다.
func TableQuery(expr string) *prometheus.DataqueryBuilder {
	return Query(expr).
		Format(prometheus.PromQueryFormatTable).
		Instant()
}

// Base 는 임계값의 첫 단계(-Infinity 부터)입니다.
func Base(color string) dashboard.Threshold {
	return dashboard.Threshold{Color: color}
}

// Step 은 value 이상일 때 적용되는 임계값 단계입니다.
func Step(value float64, color string) dashboard.Threshold {
	return dashboard.Threshold{Value: cog.ToPtr(value), Color: color}
}

// Thresholds 는 절대값 기준 임계값 설정을 만듭니다.
func Thresholds(steps ...dashboard.Threshold) *dashboard.ThresholdsConfigBuilder {
	return dashboard.NewThresholdsConfigBuilder().
		Mode(dashboard.ThresholdsModeAbsolute).
		Steps(steps)
}

// ColorMode 는 필드 색상 모드를 만듭니다.
func ColorMode(mode dashboard.FieldColorModeId) *dashboard.FieldColorBuilder {
	return dashboard.NewFieldColorBuilder().Mode(mode)
}

// ---------------------------------------------------------------------------
// 패널 기본값
// ---------------------------------------------------------------------------

// Stat 은 마지막 값(lastNotNull)을 크게 보여주는 Stat 패널의 기본형입니다.
func Stat(title, unit string) *stat.PanelBuilder {
	return stat.NewPanelBuilder().
		Title(title).
		Datasource(Datasource()).
		Unit(unit).
		ColorScheme(ColorMode(dashboard.FieldColorModeIdThresholds)).
		ColorMode(sdkcommon.BigValueColorModeValue).
		GraphMode(sdkcommon.BigValueGraphModeArea).
		JustifyMode(sdkcommon.BigValueJustifyModeAuto).
		TextMode(sdkcommon.BigValueTextModeAuto).
		ReduceOptions(sdkcommon.NewReduceDataOptionsBuilder().
			Values(false).
			Calcs([]string{"lastNotNull"}))
}

// Timeseries 는 선 그래프 기본형입니다. 범례는 표 형태로 하단에, 툴팁은 전체 시리즈를 내림차순으로 보여줍니다.
func Timeseries(title, unit string) *timeseries.PanelBuilder {
	return timeseries.NewPanelBuilder().
		Title(title).
		Datasource(Datasource()).
		Unit(unit).
		ColorScheme(ColorMode(dashboard.FieldColorModeIdPaletteClassic)).
		DrawStyle(sdkcommon.GraphDrawStyleLine).
		LineWidth(1).
		FillOpacity(10).
		ShowPoints(sdkcommon.VisibilityModeNever).
		SpanNulls(sdkcommon.BoolOrFloat64{Bool: cog.ToPtr(true)}).
		Legend(sdkcommon.NewVizLegendOptionsBuilder().
			DisplayMode(sdkcommon.LegendDisplayModeTable).
			Placement(sdkcommon.LegendPlacementBottom).
			ShowLegend(true).
			Calcs([]string{"mean", "max", "lastNotNull"})).
		Tooltip(sdkcommon.NewVizTooltipOptionsBuilder().
			Mode(sdkcommon.TooltipDisplayModeMulti).
			Sort(sdkcommon.SortOrderDescending))
}

// ---------------------------------------------------------------------------
// 빌드
// ---------------------------------------------------------------------------

// Build 는 대시보드를 빌드한 뒤, SDK 가 채워주지 않는 값들을 후처리합니다.
//
//   - 패널 ID: SDK 는 ID 를 자동으로 붙이지 않습니다. Grafana 는 패널 링크(viewPanel=ID),
//     패널 복제 등에서 ID 를 사용하므로, 위에서부터 순서대로 1, 2, 3... 을 부여합니다.
//     코드에서 패널 순서만 유지되면 ID 도 매번 같게 나오므로 diff 가 안정적입니다.
//   - schemaVersion: 대상 Grafana 버전에 맞춥니다. (SchemaVersion 상수 참고)
func Build(builder *dashboard.DashboardBuilder) (dashboard.Dashboard, error) {
	dash, err := builder.Build()
	if err != nil {
		return dashboard.Dashboard{}, err
	}

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

	dash.SchemaVersion = SchemaVersion

	return dash, nil
}
