// convert 는 대시보드 JSON 을 Foundation SDK Go 코드 "초안"으로 바꿔줍니다.
// grafanalib 이 만든 JSON 도 그대로 넣을 수 있어서, grafanalib → Go 마이그레이션의 출발점으로 씁니다.
//
//	go run ./cmd/convert -o ../dist/grafanalib-to-go.draft.txt ../dist/grafanalib.normalized.json
//	go run ./cmd/convert ../dist/grafanalib.normalized.json        (화면에 출력)
//
// 출력은 gofmt 도 안 된 긴 빌더 체인이고, 레거시 필드와 기본값까지 전부 들어 있습니다.
// 그대로 쓰지 말고 참고용으로 보면서 main.go 처럼 정리하세요.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/grafana/grafana-foundation-sdk/go/cog/plugins"
	"github.com/grafana/grafana-foundation-sdk/go/dashboard"
)

func main() {
	output := flag.String("o", "", "저장할 파일 경로. 없으면 화면에 출력합니다.")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: convert [-o draft.txt] <dashboard.json>")
		os.Exit(2)
	}

	// 패널/쿼리 타입별 unmarshal 을 위해 기본 플러그인을 등록해야 합니다.
	plugins.RegisterDefaultPlugins()

	raw, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	dash := dashboard.Dashboard{}
	if err := json.Unmarshal(raw, &dash); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	code := dashboard.DashboardConverter(dash) + "\n"
	if *output == "" {
		fmt.Print(code)
		return
	}
	if err := os.WriteFile(*output, []byte(code), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println("generated", *output)
}
