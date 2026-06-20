// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"strings"
	"testing"

	"github.com/mendixlabs/mxcli/mdl/ast"
)

// Issue #650 — MDL-WIDGET04 flags a dynamictext whose template references a {N}
// placeholder with no matching parameter binding (orphaned ClientTemplate).
func TestValidateDynamicTextPlaceholders(t *testing.T) {
	dt := func(props map[string]any) *ast.WidgetV3 {
		return &ast.WidgetV3{Type: "dynamictext", Name: "txt", Properties: props}
	}
	cases := []struct {
		name    string
		widget  *ast.WidgetV3
		wantBad bool
	}{
		{"orphan {1}", dt(map[string]any{"Content": "{1}"}), true},
		{"orphan {2} with one param", dt(map[string]any{
			"Content":       "Hi {1} {2}",
			"ContentParams": []ast.ParamAssignmentV3{{Value: "Name"}},
		}), true},
		{"bound via Attribute", dt(map[string]any{"Content": "{1}", "Attribute": "Title"}), false},
		{"bound via ContentParams", dt(map[string]any{
			"Content":       "{1}",
			"ContentParams": []ast.ParamAssignmentV3{{Value: "Title"}},
		}), false},
		{"static text, no placeholder", dt(map[string]any{"Content": "Hello"}), false},
		{"empty content (no AST placeholder)", dt(map[string]any{}), false},
		{"not a dynamictext", &ast.WidgetV3{Type: "textbox", Name: "tb", Properties: map[string]any{"Content": "{1}"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := validateDynamicTextPlaceholders(c.widget, "page X")
			if c.wantBad && v == nil {
				t.Errorf("expected MDL-WIDGET04 violation, got none")
			}
			if !c.wantBad && v != nil {
				t.Errorf("unexpected violation: %s", v.Message)
			}
			if v != nil && v.RuleID != "MDL-WIDGET04" {
				t.Errorf("RuleID = %s, want MDL-WIDGET04", v.RuleID)
			}
			if c.wantBad && v != nil && !strings.Contains(v.Message, "orphaned placeholder") {
				t.Errorf("message lacks guidance: %s", v.Message)
			}
		})
	}
}
