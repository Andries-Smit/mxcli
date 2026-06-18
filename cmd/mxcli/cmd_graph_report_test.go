// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderGraphMarkdown(t *testing.T) {
	sections := []graphSectionResult{
		{Title: "God nodes", Columns: []string{"Asset", "Degree"}, Rows: [][]any{
			{"M.Order", int64(42)},
			{"M.Pipe|Sep", int64(7)}, // pipe must be escaped
		}},
		{Title: "Empty", Columns: []string{"X"}, Rows: nil},
	}
	out := renderGraphMarkdown("app.mpr", 15, false, sections)

	for _, want := range []string{
		"# Graph report — app.mpr",
		"framework modules excluded",
		"## God nodes",
		"| Asset | Degree |",
		"| M.Order | 42 |",
		`| M.Pipe\|Sep | 7 |`, // escaped pipe
		"## Empty",
		"_(none)_",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown missing %q:\n%s", want, out)
		}
	}
}

func TestRenderGraphJSON(t *testing.T) {
	sections := []graphSectionResult{
		{Title: "God nodes", Columns: []string{"Asset", "Degree"}, Rows: [][]any{{"M.Order", int64(42)}}},
	}
	out := renderGraphJSON("app.mpr", sections)

	var parsed struct {
		Project  string `json:"project"`
		Sections []struct {
			Title string           `json:"title"`
			Rows  []map[string]any `json:"rows"`
		} `json:"sections"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if parsed.Project != "app.mpr" || len(parsed.Sections) != 1 {
		t.Fatalf("unexpected shape: %+v", parsed)
	}
	if got := parsed.Sections[0].Rows[0]["Asset"]; got != "M.Order" {
		t.Errorf("Asset = %v, want M.Order", got)
	}
}
