package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/prometheus/prometheus/promql/parser"
)

// 대시보드 JSON 을 Grafana 에 올리기 전에 잡을 수 있는 실수들을 검사합니다.
//
//   - 빌드 에러 (SDK 빌더 검증)
//   - UID 누락 / 중복
//   - 패널 ID 중복, 패널 겹침, 그리드(24칸) 초과
//   - 한 패널 안에서 refId 중복
//   - PromQL 문법 오류 (Prometheus 공식 파서로 파싱)
//
// go test ./... 로 실행합니다. (make lint 에 포함)

// grafanaVars 는 PromQL 파싱 전에 Grafana 변수를 그럴듯한 값으로 치환하기 위한 목록입니다.
// 대시보드에 새 변수를 추가했다면 여기에도 추가하세요. (치환 안 된 $변수가 남으면 테스트가 실패합니다)
var grafanaVars = strings.NewReplacer(
	"$__rate_interval", "1m",
	"$__interval", "1m",
	"$rate_window", "5m",
	"$namespace", "default",
	"$workload", "app",
	"$pod", ".*",
	"$krw", "1380",
	"$homelab_cost", "29000",
)

// panel 은 테스트에 필요한 필드만 가진 패널 JSON 입니다.
type panel struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	GridPos struct {
		X, Y, W, H int
	} `json:"gridPos"`
	Targets []struct {
		RefID string `json:"refId"`
		Expr  string `json:"expr"`
	} `json:"targets"`
	Panels []panel `json:"panels"`
}

func TestDashboards(t *testing.T) {
	seenUIDs := map[string]string{}

	for name, build := range dashboards {
		t.Run(name, func(t *testing.T) {
			dash, err := build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if dash.Uid == nil || *dash.Uid == "" {
				t.Fatal("uid 가 비어 있습니다")
			}
			if other, ok := seenUIDs[*dash.Uid]; ok {
				t.Fatalf("uid %q 가 %s 와 중복됩니다", *dash.Uid, other)
			}
			seenUIDs[*dash.Uid] = name

			raw, err := json.Marshal(dash)
			if err != nil {
				t.Fatal(err)
			}
			var parsed struct {
				Panels []panel `json:"panels"`
			}
			if err := json.Unmarshal(raw, &parsed); err != nil {
				t.Fatal(err)
			}

			var all []panel
			for _, p := range parsed.Panels {
				all = append(all, p)
				all = append(all, p.Panels...)
			}

			checkPanelIDs(t, all)
			checkLayout(t, all)
			for _, p := range all {
				checkTargets(t, p)
			}
		})
	}
}

func checkPanelIDs(t *testing.T, panels []panel) {
	t.Helper()
	seen := map[int]string{}
	for _, p := range panels {
		if p.ID == 0 {
			t.Errorf("%q: 패널 ID 가 없습니다", p.Title)
		}
		if other, ok := seen[p.ID]; ok {
			t.Errorf("패널 ID %d 가 %q 와 %q 에서 중복됩니다", p.ID, other, p.Title)
		}
		seen[p.ID] = p.Title
	}
}

func checkLayout(t *testing.T, panels []panel) {
	t.Helper()
	for i, a := range panels {
		if a.GridPos.W <= 0 || a.GridPos.H <= 0 || a.GridPos.X+a.GridPos.W > 24 {
			t.Errorf("%q: 잘못된 gridPos %+v", a.Title, a.GridPos)
		}
		for _, b := range panels[i+1:] {
			if overlaps(a, b) {
				t.Errorf("%q 와 %q 가 겹칩니다 (%+v / %+v)", a.Title, b.Title, a.GridPos, b.GridPos)
			}
		}
	}
}

func overlaps(a, b panel) bool {
	return a.GridPos.X < b.GridPos.X+b.GridPos.W && b.GridPos.X < a.GridPos.X+a.GridPos.W &&
		a.GridPos.Y < b.GridPos.Y+b.GridPos.H && b.GridPos.Y < a.GridPos.Y+a.GridPos.H
}

func checkTargets(t *testing.T, p panel) {
	t.Helper()
	if p.Type != "row" && len(p.Targets) == 0 {
		t.Errorf("%q: 쿼리가 없습니다", p.Title)
	}

	refIDs := map[string]bool{}
	for _, target := range p.Targets {
		if target.RefID == "" || refIDs[target.RefID] {
			t.Errorf("%q: refId %q 가 비어 있거나 중복됩니다", p.Title, target.RefID)
		}
		refIDs[target.RefID] = true

		expr := grafanaVars.Replace(target.Expr)
		if strings.Contains(expr, "$") {
			t.Errorf("%q/%s: 치환되지 않은 Grafana 변수가 있습니다 (grafanaVars 에 추가하세요): %s", p.Title, target.RefID, expr)
			continue
		}
		if _, err := parser.ParseExpr(expr); err != nil {
			t.Errorf("%q/%s: PromQL 파싱 실패: %v\n%s", p.Title, target.RefID, err, target.Expr)
		}
	}
}
