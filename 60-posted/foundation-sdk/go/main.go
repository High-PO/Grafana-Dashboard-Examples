// 대시보드 생성기.
//
// 등록된 대시보드를 모두 빌드해서 <out>/<name>.json 으로 저장합니다.
// Terraform(60-posted/terraform)은 이 디렉토리의 JSON 을 그대로 읽어서 Grafana 에 올립니다.
//
//	go run . -out ../../dist
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/grafana/grafana-foundation-sdk/go/dashboard"

	"github.com/High-PO/Grafana-Dashboard-Examples/60-posted/foundation-sdk/go/dashboards/finops"
	"github.com/High-PO/Grafana-Dashboard-Examples/60-posted/foundation-sdk/go/dashboards/servicehealth"
)

// dashboards 는 생성할 대시보드 목록입니다. 키는 출력 파일 이름이자 Terraform 리소스 키가 됩니다.
// 새 대시보드를 추가하면 여기에 한 줄만 등록하면 됩니다.
var dashboards = map[string]func() (dashboard.Dashboard, error){
	"service-health":  servicehealth.Build,
	"finops-opencost": finops.Build,
}

func main() {
	out := flag.String("out", "../../dist", "생성된 JSON 을 저장할 디렉토리")
	flag.Parse()

	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}

	// 코드에서 지운 대시보드의 JSON 이 남아 있으면 Terraform 이 계속 관리하게 되므로 먼저 비웁니다.
	stale, err := filepath.Glob(filepath.Join(out, "*.json"))
	if err != nil {
		return err
	}
	for _, f := range stale {
		if err := os.Remove(f); err != nil {
			return err
		}
	}

	names := make([]string, 0, len(dashboards))
	for name := range dashboards {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		dash, err := dashboards[name]()
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}

		raw, err := json.MarshalIndent(dash, "", "  ")
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}

		path := filepath.Join(out, name+".json")
		if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Println("generated", path)
	}

	return nil
}
