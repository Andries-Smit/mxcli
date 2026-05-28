// SPDX-License-Identifier: Apache-2.0

package types

import "testing"

func TestWidgetVisibilityConditionHidden(t *testing.T) {
	values := map[string]string{
		"type":                "expression",
		"customVisualization": "true",
		"groupEvents":         "false",
		"empty":               "",
		"zero":                "0",
	}

	tests := []struct {
		name string
		cond *WidgetVisibilityCondition
		want bool
	}{
		{"nil condition", nil, false},
		{"eq match", &WidgetVisibilityCondition{PropertyKey: "type", Operator: "eq", Value: "expression"}, true},
		{"eq no match", &WidgetVisibilityCondition{PropertyKey: "type", Operator: "eq", Value: "dynamic"}, false},
		{"ne match", &WidgetVisibilityCondition{PropertyKey: "type", Operator: "ne", Value: "dynamic"}, true},
		{"ne no match", &WidgetVisibilityCondition{PropertyKey: "type", Operator: "ne", Value: "expression"}, false},
		{"truthy true-string", &WidgetVisibilityCondition{PropertyKey: "customVisualization", Operator: "truthy"}, true},
		{"truthy false-string", &WidgetVisibilityCondition{PropertyKey: "groupEvents", Operator: "truthy"}, false},
		{"truthy empty", &WidgetVisibilityCondition{PropertyKey: "empty", Operator: "truthy"}, false},
		{"truthy zero", &WidgetVisibilityCondition{PropertyKey: "zero", Operator: "truthy"}, false},
		{"falsy false-string", &WidgetVisibilityCondition{PropertyKey: "groupEvents", Operator: "falsy"}, true},
		{"falsy true-string", &WidgetVisibilityCondition{PropertyKey: "customVisualization", Operator: "falsy"}, false},
		{"falsy missing key", &WidgetVisibilityCondition{PropertyKey: "absent", Operator: "falsy"}, true},
		{"unknown operator", &WidgetVisibilityCondition{PropertyKey: "type", Operator: "regex", Value: "x"}, false},
		{"empty operator", &WidgetVisibilityCondition{PropertyKey: "type", Operator: ""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cond.Hidden(values); got != tt.want {
				t.Errorf("Hidden() = %v, want %v", got, tt.want)
			}
		})
	}
}
