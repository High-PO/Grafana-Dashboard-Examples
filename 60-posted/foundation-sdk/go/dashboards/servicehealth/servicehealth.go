// Package servicehealth 는 새로 만든 예제 대시보드입니다.
//
// 특정 서비스(= namespace + workload)를 골라서 CPU / Memory / Disk I/O 상태를 한 화면에서 봅니다.
// 사용하는 메트릭:
//   - cAdvisor(kubelet)      : container_cpu_*, container_memory_*, container_fs_*
//   - kube-state-metrics     : kube_pod_*, kube_pod_container_*
//   - kubelet volume stats   : kubelet_volume_stats_*
//
// kube-prometheus-stack 을 설치했다면 별도 설정 없이 모두 수집되고 있습니다.
package servicehealth

import (
	"fmt"

	"github.com/grafana/grafana-foundation-sdk/go/bargauge"
	"github.com/grafana/grafana-foundation-sdk/go/cog"
	sdkcommon "github.com/grafana/grafana-foundation-sdk/go/common"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
	"github.com/grafana/grafana-foundation-sdk/go/stat"
	"github.com/grafana/grafana-foundation-sdk/go/timeseries"

	"github.com/High-PO/Grafana-Dashboard-Examples/60-posted/foundation-sdk/go/internal/common"
)

const (
	UID   = "service-health"
	Title = "Service Health — CPU / Memory / Disk I/O"
)

// PromQL 에서 반복되는 라벨 셀렉터들.
const (
	// kube-state-metrics 메트릭용 (container 라벨이 없거나 의미가 다름)
	podSel = `namespace="$namespace", pod=~"$pod"`
	// cAdvisor 메트릭용. container="" 는 파드 cgroup 합계, container="POD" 는 pause 컨테이너라 제외합니다.
	containerSel = `namespace="$namespace", pod=~"$pod", container!="", container!="POD"`
)

// q 는 PromQL 템플릿의 %[1]s 에 containerSel, %[2]s 에 podSel 을 채워줍니다.
func q(format string) string {
	return fmt.Sprintf(format, containerSel, podSel)
}

// PromQL 모음. 패널 여러 곳에서 재사용하는 식은 여기서 한 번만 정의합니다.
var (
	cpuUsage      = q(`sum(rate(container_cpu_usage_seconds_total{%[1]s}[$rate_window]))`)
	cpuUsageByPod = q(`sum by(pod) (rate(container_cpu_usage_seconds_total{%[1]s}[$rate_window]))`)
	cpuRequest    = q(`sum(kube_pod_container_resource_requests{%[2]s, resource="cpu"})`)
	cpuLimit      = q(`sum(kube_pod_container_resource_limits{%[2]s, resource="cpu"})`)
	cpuThrottled  = q(`sum by(pod) (rate(container_cpu_cfs_throttled_periods_total{%[1]s}[$rate_window]))
/
sum by(pod) (rate(container_cpu_cfs_periods_total{%[1]s}[$rate_window]))`)

	memUsage      = q(`sum(container_memory_working_set_bytes{%[1]s})`)
	memUsageByPod = q(`sum by(pod) (container_memory_working_set_bytes{%[1]s})`)
	memRequest    = q(`sum(kube_pod_container_resource_requests{%[2]s, resource="memory"})`)
	memLimit      = q(`sum(kube_pod_container_resource_limits{%[2]s, resource="memory"})`)
	restartsByPod = q(`sum by(pod) (increase(kube_pod_container_status_restarts_total{%[2]s}[$rate_window]))`)

	diskReadBytes  = q(`sum(rate(container_fs_reads_bytes_total{%[1]s}[$rate_window]))`)
	diskWriteBytes = q(`sum(rate(container_fs_writes_bytes_total{%[1]s}[$rate_window]))`)
)

// Build 는 대시보드를 생성합니다.
func Build() (dashboard.Dashboard, error) {
	builder := dashboard.NewDashboardBuilder(Title).
		Uid(UID).
		Description("선택한 서비스(workload)의 CPU / Memory / Disk I/O 상태를 한 화면에서 봅니다. Foundation SDK(Go)로 생성됨.").
		Tags([]string{"service", "kubernetes", "foundation-sdk"}).
		Editable().
		Tooltip(dashboard.DashboardCursorSyncCrosshair).
		Refresh("30s").
		Time("now-3h", "now").
		Annotation(common.BuiltInAnnotation()).
		// 변수
		WithVariable(common.DatasourceVariable()).
		WithVariable(common.QueryVariable("namespace", "Namespace",
			`label_values(kube_pod_info, namespace)`)).
		WithVariable(common.QueryVariable("workload", "Workload",
			`label_values(kube_pod_info{namespace="$namespace"}, created_by_name)`).
			Description("Deployment / StatefulSet / DaemonSet 이름. ReplicaSet 해시는 정규식으로 잘라냅니다.").
			// Deployment 파드의 created_by_name 은 ReplicaSet 이름(<deploy>-<hash>)이라 해시를 떼어냅니다.
			// pod-template-hash 는 모음/헷갈리는 문자를 뺀 문자셋으로 만들어지므로 그 문자셋만 매칭합니다.
			Regex(`/^(.+?)(?:-[bcdfghjklmnpqrstvwxz2456789]{6,10})?$/`)).
		WithVariable(common.QueryVariable("pod", "Pod",
			`label_values(kube_pod_info{namespace="$namespace", pod=~"$workload-.*"}, pod)`).
			Multi(true).
			IncludeAll(true).
			Current(dashboard.VariableOption{
				Text:  dashboard.StringOrArrayOfString{String: cog.ToPtr("All")},
				Value: dashboard.StringOrArrayOfString{ArrayOfString: []string{"$__all"}},
			})).
		WithVariable(common.CustomVariable("rate_window", "Rate 윈도우",
			[]string{"1m", "2m", "5m", "10m"}, "5m")).
		// 패널
		WithRow(dashboard.NewRowBuilder("요약")).
		WithPanel(readyPods()).
		WithPanel(restarts()).
		WithPanel(cpuNow()).
		WithPanel(cpuVsRequest()).
		WithPanel(memNow()).
		WithPanel(memVsLimit()).
		WithPanel(diskNow("Disk Read", diskReadBytes)).
		WithPanel(diskNow("Disk Write", diskWriteBytes)).
		WithRow(dashboard.NewRowBuilder("CPU")).
		WithPanel(cpuByPod()).
		WithPanel(cpuTotal()).
		WithPanel(cpuThrottling()).
		WithRow(dashboard.NewRowBuilder("Memory")).
		WithPanel(memByPod()).
		WithPanel(memTotal()).
		WithPanel(restartsTimeline()).
		WithRow(dashboard.NewRowBuilder("Disk I/O")).
		WithPanel(diskThroughput()).
		WithPanel(diskIOPS()).
		WithPanel(pvcUsage())

	return common.Build(builder)
}

// ---------------------------------------------------------------------------
// 요약 (Stat 8개, 한 줄에 3칸씩)
// ---------------------------------------------------------------------------

func summaryStat(title, unit, expr string) *stat.PanelBuilder {
	return common.Stat(title, unit).
		Span(3).
		Height(4).
		WithTarget(common.Query(expr).RefId("A"))
}

func readyPods() *stat.PanelBuilder {
	return summaryStat("Ready 파드", "none",
		q(`sum(kube_pod_status_ready{%[2]s, condition="true"})`)).
		Description("Ready 상태인 파드 수. 0 이면 서비스가 트래픽을 받을 수 없습니다.").
		Decimals(0).
		GraphMode(sdkcommon.BigValueGraphModeNone).
		Thresholds(common.Thresholds(common.Base("red"), common.Step(1, "green")))
}

func restarts() *stat.PanelBuilder {
	return summaryStat("재시작 (1h)", "none",
		q(`sum(increase(kube_pod_container_status_restarts_total{%[2]s}[1h]))`)).
		Description("최근 1시간 동안 컨테이너 재시작 횟수 합계. CrashLoop / OOMKilled 여부를 먼저 확인하세요.").
		Decimals(0).
		GraphMode(sdkcommon.BigValueGraphModeNone).
		Thresholds(common.Thresholds(common.Base("green"), common.Step(1, "orange"), common.Step(3, "red")))
}

func cpuNow() *stat.PanelBuilder {
	return summaryStat("CPU 사용량 (core)", "none", cpuUsage).
		Decimals(2).
		Thresholds(common.Thresholds(common.Base("blue")))
}

func cpuVsRequest() *stat.PanelBuilder {
	return summaryStat("CPU / Request", "percentunit", cpuUsage+"\n/\n"+cpuRequest).
		Description("Request 대비 사용률. 100% 를 넘으면 Request 를 너무 작게 잡은 것입니다.").
		Decimals(0).
		Thresholds(common.Thresholds(common.Base("green"), common.Step(0.8, "orange"), common.Step(1, "red")))
}

func memNow() *stat.PanelBuilder {
	return summaryStat("Memory (Working Set)", "bytes", memUsage).
		Thresholds(common.Thresholds(common.Base("blue")))
}

func memVsLimit() *stat.PanelBuilder {
	return summaryStat("Memory / Limit", "percentunit", memUsage+"\n/\n"+memLimit).
		Description("Limit 대비 Working Set. 100% 에 닿으면 OOMKilled 됩니다. Limit 이 없으면 값이 비어 있습니다.").
		Decimals(0).
		Thresholds(common.Thresholds(common.Base("green"), common.Step(0.8, "orange"), common.Step(0.9, "red")))
}

func diskNow(title, expr string) *stat.PanelBuilder {
	return summaryStat(title, "Bps", expr).
		Thresholds(common.Thresholds(common.Base("purple")))
}

// ---------------------------------------------------------------------------
// CPU
// ---------------------------------------------------------------------------

func cpuByPod() *timeseries.PanelBuilder {
	return common.Timeseries("CPU 사용량 (파드별)", "none").
		Span(8).
		Height(8).
		Decimals(2).
		Min(0).
		WithTarget(common.Query(cpuUsageByPod).LegendFormat("{{pod}}").RefId("A"))
}

func cpuTotal() *timeseries.PanelBuilder {
	return common.Timeseries("CPU 합계 vs Request / Limit", "none").
		Description("서비스 전체 CPU 사용량과 Request / Limit 합계를 비교합니다.").
		Span(8).
		Height(8).
		Decimals(2).
		Min(0).
		WithTarget(common.Query(cpuUsage).LegendFormat("사용량").RefId("A")).
		WithTarget(common.Query(cpuRequest).LegendFormat("Request").RefId("B")).
		WithTarget(common.Query(cpuLimit).LegendFormat("Limit").RefId("C")).
		OverrideByName("Request", referenceLine("orange")).
		OverrideByName("Limit", referenceLine("red"))
}

func cpuThrottling() *timeseries.PanelBuilder {
	return common.Timeseries("CPU 스로틀링 비율 (파드별)", "percentunit").
		Description("CFS period 중 스로틀링된 비율. 25% 이상이 지속되면 CPU Limit 상향을 고려하세요.").
		Span(8).
		Height(8).
		Min(0).
		Max(1).
		Thresholds(common.Thresholds(common.Base("green"), common.Step(0.25, "red"))).
		ThresholdsStyle(sdkcommon.NewGraphThresholdsStyleConfigBuilder().
			Mode(sdkcommon.GraphThresholdsStyleModeDashed)).
		WithTarget(common.Query(cpuThrottled).LegendFormat("{{pod}}").RefId("A"))
}

// ---------------------------------------------------------------------------
// Memory
// ---------------------------------------------------------------------------

func memByPod() *timeseries.PanelBuilder {
	return common.Timeseries("Memory Working Set (파드별)", "bytes").
		Span(8).
		Height(8).
		Min(0).
		WithTarget(common.Query(memUsageByPod).LegendFormat("{{pod}}").RefId("A"))
}

func memTotal() *timeseries.PanelBuilder {
	return common.Timeseries("Memory 합계 vs Request / Limit", "bytes").
		Span(8).
		Height(8).
		Min(0).
		WithTarget(common.Query(memUsage).LegendFormat("사용량").RefId("A")).
		WithTarget(common.Query(memRequest).LegendFormat("Request").RefId("B")).
		WithTarget(common.Query(memLimit).LegendFormat("Limit").RefId("C")).
		OverrideByName("Request", referenceLine("orange")).
		OverrideByName("Limit", referenceLine("red"))
}

func restartsTimeline() *timeseries.PanelBuilder {
	return common.Timeseries("컨테이너 재시작 (파드별)", "none").
		Description("rate 윈도우 동안 발생한 재시작 횟수. 막대가 보이는 시점의 로그/이벤트를 확인하세요.").
		Span(8).
		Height(8).
		Decimals(0).
		Min(0).
		DrawStyle(sdkcommon.GraphDrawStyleBars).
		FillOpacity(80).
		WithTarget(common.Query(restartsByPod).LegendFormat("{{pod}}").RefId("A"))
}

// ---------------------------------------------------------------------------
// Disk I/O
// ---------------------------------------------------------------------------

// readWritePanel 은 읽기는 위(+), 쓰기는 아래(-)로 그리는 그래프입니다.
func readWritePanel(title, unit, readMetric, writeMetric string) *timeseries.PanelBuilder {
	byPod := func(metric string) string {
		return fmt.Sprintf(`sum by(pod) (rate(%s{%s}[$rate_window]))`, metric, containerSel)
	}

	return common.Timeseries(title, unit).
		Description("위쪽(+)은 읽기, 아래쪽(-)은 쓰기입니다.").
		Span(12).
		Height(8).
		WithTarget(common.Query(byPod(readMetric)).LegendFormat("{{pod}} read").RefId("A")).
		WithTarget(common.Query(byPod(writeMetric)).LegendFormat("{{pod}} write").RefId("B")).
		OverrideByRegexp(".* write$", []dashboard.DynamicConfigValue{
			{Id: "custom.transform", Value: "negative-Y"},
		})
}

func diskThroughput() *timeseries.PanelBuilder {
	return readWritePanel("Disk 처리량 (파드별)", "Bps",
		"container_fs_reads_bytes_total", "container_fs_writes_bytes_total")
}

func diskIOPS() *timeseries.PanelBuilder {
	return readWritePanel("Disk IOPS (파드별)", "iops",
		"container_fs_reads_total", "container_fs_writes_total")
}

func pvcUsage() *bargauge.PanelBuilder {
	return bargauge.NewPanelBuilder().
		Title("PVC 사용률 (네임스페이스)").
		Description("네임스페이스의 PVC 사용률입니다. PVC 는 파드와 직접 연결된 라벨이 없어 namespace 단위로 보여줍니다.").
		Datasource(common.Datasource()).
		Span(24).
		Height(6).
		Unit("percentunit").
		Min(0).
		Max(1).
		Thresholds(common.Thresholds(common.Base("green"), common.Step(0.75, "orange"), common.Step(0.9, "red"))).
		ColorScheme(common.ColorMode(dashboard.FieldColorModeIdThresholds)).
		DisplayMode(sdkcommon.BarGaugeDisplayModeGradient).
		Orientation(sdkcommon.VizOrientationHorizontal).
		ReduceOptions(sdkcommon.NewReduceDataOptionsBuilder().
			Values(false).
			Calcs([]string{"lastNotNull"})).
		WithTarget(common.Query(`max by(persistentvolumeclaim) (kubelet_volume_stats_used_bytes{namespace="$namespace"})
/
max by(persistentvolumeclaim) (kubelet_volume_stats_capacity_bytes{namespace="$namespace"})`).
			LegendFormat("{{persistentvolumeclaim}}").
			RefId("A"))
}

// referenceLine 은 Request / Limit 처럼 "기준선" 역할을 하는 시리즈를 점선으로 그립니다.
func referenceLine(color string) []dashboard.DynamicConfigValue {
	return []dashboard.DynamicConfigValue{
		{Id: "color", Value: map[string]any{"mode": "fixed", "fixedColor": color}},
		{Id: "custom.lineStyle", Value: map[string]any{"fill": "dash", "dash": []int{10, 10}}},
		{Id: "custom.fillOpacity", Value: 0},
		{Id: "custom.lineWidth", Value: 2},
	}
}
