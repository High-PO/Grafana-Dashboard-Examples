// convert 는 대시보드 JSON 을 **바로 실행할 수 있는 Foundation SDK Go 코드(main.go)** 로 바꿔줍니다.
// grafanalib 이 만든 JSON 을 넣으면 grafanalib → Go 마이그레이션이 됩니다.
//
//	go run ./cmd/convert -o migrated/main.go ../dist/grafanalib.normalized.json
//	go run ./migrated -o ../dist/migrated-go.json      ← 변환된 Go 코드로 대시보드 JSON 생성
//
// SDK 의 DashboardConverter 는 빌더 체인 "식" 하나만 돌려줍니다 (package / import / main 이 없음).
// 이 도구는 그 식을 감싸서 import 를 붙이고, JSON 을 출력하는 main 함수를 만들고, gofmt 까지 적용합니다.
//
// 변환 결과에는 기본값과 레거시 값까지 전부 들어 있어서 길고 읽기 어렵습니다 (약 400줄).
// "동작하는 출발점"으로 쓰고, foundation_sdk_go/main.go 처럼 헬퍼와 상수로 정리하세요.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/grafana/grafana-foundation-sdk/go/cog/plugins"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
)

const sdk = "github.com/grafana/grafana-foundation-sdk/go/"

// 변환 결과에 나올 수 있는 SDK 패키지와 import 경로.
var sdkPackages = map[string]string{
	"cog":      sdk + "cog",
	"variants": sdk + "cog/variants",
	"common":   sdk + "common",
	"units":    sdk + "units",
}

func init() {
	for _, p := range []string{
		"dashboard", "prometheus", "loki", "tempo", "elasticsearch", "testdata", "expr",
		"stat", "timeseries", "table", "logs", "barchart", "bargauge", "gauge", "piechart",
		"text", "heatmap", "histogram", "statetimeline", "statushistory", "trend", "xychart",
		"nodegraph", "geomap", "canvas", "candlestick", "news", "dashboardlist", "annotationslist", "datagrid",
	} {
		sdkPackages[p] = sdk + p
	}
}

// pkg.Exported 형태만 찾습니다. 문자열 안의 "veth.*" 같은 것은 다음 글자가 대문자가 아니라서 걸러집니다.
var qualifier = regexp.MustCompile(`\b([a-z][a-z0-9]*)\.[A-Z]`)

const template = `// cmd/convert 가 %s 에서 변환한 코드입니다.
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

%s
)

func dashboardBuilder() *dashboard.DashboardBuilder {
	return %s
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
	dash.SchemaVersion = %d

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
`

func main() {
	output := flag.String("o", "", "저장할 Go 파일 경로 (예: migrated/main.go). 없으면 화면에 출력합니다.")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: convert [-o migrated/main.go] <dashboard.json>")
		os.Exit(2)
	}
	src := flag.Arg(0)

	// 패널/쿼리 타입별 unmarshal 을 위해 기본 플러그인을 등록해야 합니다.
	plugins.RegisterDefaultPlugins()

	raw, err := os.ReadFile(src)
	if err != nil {
		fail(err)
	}

	dash := dashboard.Dashboard{}
	if err := json.Unmarshal(raw, &dash); err != nil {
		fail(err)
	}

	// 1) JSON → 빌더 체인 식 (SDK 기본 기능)
	expr := dashboard.DashboardConverter(dash)

	// 2) 식에서 쓰는 SDK 패키지를 찾아 import 목록 만들기
	used := map[string]bool{}
	for _, m := range qualifier.FindAllStringSubmatch(expr, -1) {
		if path, ok := sdkPackages[m[1]]; ok {
			used[path] = true
		}
	}
	imports := make([]string, 0, len(used))
	for path := range used {
		imports = append(imports, fmt.Sprintf("\t%q", path))
	}
	sort.Strings(imports)

	// 3) package main + import + main() 으로 감싸고 gofmt
	code := fmt.Sprintf(template, strings.ReplaceAll(src, `\`, "/"), strings.Join(imports, "\n"), expr, dash.SchemaVersion)
	formatted, err := format.Source([]byte(code))
	if err != nil {
		fail(fmt.Errorf("gofmt 실패 (변환 결과가 올바른 Go 문법이 아님): %w", err))
	}

	if *output == "" {
		os.Stdout.Write(formatted)
		return
	}
	if dir := dirOf(*output); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fail(err)
		}
	}
	if err := os.WriteFile(*output, formatted, 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("generated %s (%d줄)\n", *output, strings.Count(string(formatted), "\n"))
}

func dirOf(path string) string {
	i := strings.LastIndexAny(path, `/\`)
	if i < 0 {
		return ""
	}
	return path[:i]
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
