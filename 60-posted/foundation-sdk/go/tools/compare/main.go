// compare 는 두 대시보드 JSON 에서 "의미 있는 필드"만 뽑아서 비교합니다.
//
//	go run ./tools/compare <원본.json> <생성.json>
//
// UI 에서 export 한 JSON 과 SDK 로 생성한 JSON 은 기본값 표기, 키 순서, 패널 ID,
// pluginVersion 같은 부분이 달라서 그냥 diff 하면 노이즈가 너무 많습니다.
// 그래서 실제 화면/동작에 영향을 주는 값(쿼리, 단위, 임계값, 배치, 변수 등)만 비교합니다.
// 차이가 있으면 목록을 출력하고 exit code 1 로 종료합니다.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
)

type obj = map[string]any

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: compare <original.json> <generated.json>")
		os.Exit(2)
	}

	want, err := project(os.Args[1])
	if err != nil {
		fail(err)
	}
	got, err := project(os.Args[2])
	if err != nil {
		fail(err)
	}

	diffs := diff("", want, got)
	if len(diffs) == 0 {
		fmt.Println("OK: 의미 있는 필드가 모두 같습니다.")
		return
	}

	fmt.Printf("%d개 차이 발견 (- 원본 / + 생성):\n", len(diffs))
	for _, d := range diffs {
		fmt.Println(d)
	}
	os.Exit(1)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(2)
}

// project 는 대시보드 JSON 에서 비교할 필드만 골라낸 구조를 만듭니다.
func project(path string) (obj, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d obj
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	out := pick(d, "uid", "title", "description", "tags", "refresh", "time", "graphTooltip", "timezone")

	variables := obj{}
	if templating, ok := d["templating"].(obj); ok {
		list, _ := templating["list"].([]any)
		for _, v := range list {
			v := v.(obj)
			p := pick(v, "type", "label", "description", "regex", "multi", "includeAll", "allValue", "definition")
			// 커스텀 변수는 query 가 문자열, 쿼리 변수는 {query, refId} 객체입니다.
			switch q := v["query"].(type) {
			case string:
				p["query"] = q
			case obj:
				p["query"] = q["query"]
			}
			if cur, ok := v["current"].(obj); ok {
				p["current"] = cur["value"]
			}
			variables[fmt.Sprint(v["name"])] = p
		}
	}
	out["variables"] = variables

	panels := obj{}
	var walk func(list []any)
	walk = func(list []any) {
		for _, p := range list {
			p := p.(obj)
			key := fmt.Sprintf("[%s] %v", p["type"], p["title"])
			if p["type"] == "row" {
				panels[key] = pick(p, "gridPos", "collapsed")
				nested, _ := p["panels"].([]any)
				walk(nested)
				continue
			}
			panels[key] = projectPanel(p)
		}
	}
	list, _ := d["panels"].([]any)
	walk(list)
	out["panels"] = panels

	return clean(out).(obj), nil
}

func projectPanel(p obj) obj {
	out := pick(p, "gridPos", "description", "transformations")

	if fc, ok := p["fieldConfig"].(obj); ok {
		if defaults, ok := fc["defaults"].(obj); ok {
			out["defaults"] = pick(defaults, "unit", "decimals", "min", "max", "thresholds", "color")
		}
		if overrides, ok := fc["overrides"].([]any); ok && len(overrides) > 0 {
			out["overrides"] = overrides
		}
	}

	targets := obj{}
	list, _ := p["targets"].([]any)
	for _, t := range list {
		t := t.(obj)
		tp := pick(t, "expr", "legendFormat", "format", "instant")
		targets[fmt.Sprint(t["refId"])] = tp
	}
	out["targets"] = targets

	return out
}

func pick(src obj, keys ...string) obj {
	out := obj{}
	for _, k := range keys {
		if v, ok := src[k]; ok {
			out[k] = v
		}
	}
	return out
}

// clean 은 null, false, 빈 문자열, 빈 배열/객체를 제거합니다.
// Grafana 는 이 값들과 "필드 없음"을 똑같이 취급하므로, 기본값을 생략했는지 여부의 차이를 없앱니다.
func clean(v any) any {
	switch t := v.(type) {
	case obj:
		out := obj{}
		for k, val := range t {
			c := clean(val)
			if isEmpty(c) {
				continue
			}
			out[k] = c
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, val := range t {
			out = append(out, clean(val))
		}
		return out
	default:
		return v
	}
}

func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case bool:
		return !t
	case string:
		return t == ""
	case obj:
		return len(t) == 0
	case []any:
		return len(t) == 0
	}
	return false
}

func diff(path string, want, got any) []string {
	wm, wok := want.(obj)
	gm, gok := got.(obj)
	if wok && gok {
		keys := map[string]bool{}
		for k := range wm {
			keys[k] = true
		}
		for k := range gm {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)

		var out []string
		for _, k := range sorted {
			out = append(out, diff(strings.TrimPrefix(path+"."+k, "."), wm[k], gm[k])...)
		}
		return out
	}

	if reflect.DeepEqual(want, got) {
		return nil
	}
	return []string{fmt.Sprintf("  %s\n    - %s\n    + %s", path, dump(want), dump(got))}
}

func dump(v any) string {
	if v == nil {
		return "(없음)"
	}
	b, _ := json.Marshal(v)
	return string(b)
}
