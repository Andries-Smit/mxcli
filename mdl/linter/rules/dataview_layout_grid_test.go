// SPDX-License-Identifier: Apache-2.0

package rules

import "testing"

func textbox(name string) map[string]any {
	return map[string]any{"$Type": "Forms$TextBox", "Name": name}
}

// dataView builds a DataView BSON node with the given direct child widgets. The
// data source is irrelevant to the rule, so it's omitted.
func dataView(name string, children ...any) map[string]any {
	return map[string]any{"$Type": "Forms$DataView", "Name": name, "Widgets": children}
}

func TestIsFormDataView(t *testing.T) {
	// A DataView with an input is a form (regardless of data source).
	if !isFormDataView(dataView("dv", textbox("tb"))) {
		t.Error("DataView with a textbox should be a form")
	}
	// An input nested in a container still counts.
	nested := dataView("dv", map[string]any{"$Type": "Forms$DivContainer", "Widgets": []any{textbox("tb")}})
	if !isFormDataView(nested) {
		t.Error("DataView with an input inside a container should be a form")
	}
	// The pluggable ComboBox counts.
	combo := map[string]any{"$Type": "CustomWidgets$CustomWidget", "Type": map[string]any{"WidgetId": "com.mendix.widget.web.combobox.Combobox"}}
	if !isFormDataView(dataView("dv", combo)) {
		t.Error("DataView with a ComboBox should be a form")
	}
	// Display-only DataView (no inputs) is NOT a form.
	display := dataView("dv", map[string]any{"$Type": "Forms$DynamicText", "Name": "t"})
	if isFormDataView(display) {
		t.Error("display-only DataView should not be a form")
	}
	// Container DataView wrapping a nested datagrid (no inputs) is NOT a form.
	wrapper := dataView("dv", map[string]any{"$Type": "Forms$DataGrid", "Name": "g"})
	if isFormDataView(wrapper) {
		t.Error("DataView wrapping a datagrid should not be a form")
	}
	// An input that lives in a *nested* DataView must not make the outer one a form.
	outer := dataView("outer", dataView("inner", textbox("tb")))
	if isFormDataView(outer) {
		t.Error("input in a nested DataView should not count for the outer DataView")
	}
	// Non-DataView is never a form.
	if isFormDataView(map[string]any{"$Type": "Forms$DivContainer", "Widgets": []any{textbox("tb")}}) {
		t.Error("non-DataView should not be a form")
	}
}

// collectReported runs the walk and returns the names of flagged DataViews.
func collectReported(root map[string]any) []string {
	var names []string
	walkForUngridedDataView(root, false, func(dv map[string]any) {
		names = append(names, widgetName(dv))
	})
	return names
}

func TestWalkForUngridedDataView_FlagsBareForm(t *testing.T) {
	root := map[string]any{"$Type": "Forms$DivContainer", "Widgets": []any{dataView("dvBare", textbox("tb"))}}
	got := collectReported(root)
	if len(got) != 1 || got[0] != "dvBare" {
		t.Fatalf("expected [dvBare], got %v", got)
	}
}

// The data source is irrelevant: a database-bound form DataView outside a grid is
// flagged just like a parameter-bound one.
func TestWalkForUngridedDataView_DatabaseFormFlagged(t *testing.T) {
	dv := dataView("dvDb", textbox("tb"))
	dv["DataSource"] = map[string]any{"$Type": "Forms$DatabaseSource"}
	root := map[string]any{"$Type": "Forms$DivContainer", "Widgets": []any{dv}}
	if got := collectReported(root); len(got) != 1 || got[0] != "dvDb" {
		t.Fatalf("expected [dvDb], got %v", got)
	}
}

func TestWalkForUngridedDataView_GridWrappedIsClean(t *testing.T) {
	root := map[string]any{
		"$Type": "Forms$LayoutGrid",
		"Rows": []any{map[string]any{
			"Columns": []any{map[string]any{"Widgets": []any{dataView("dvWrapped", textbox("tb"))}}},
		}},
	}
	if got := collectReported(root); len(got) != 0 {
		t.Fatalf("grid-wrapped form should not be flagged, got %v", got)
	}
}

func TestWalkForUngridedDataView_GridAncestorThroughContainer(t *testing.T) {
	root := map[string]any{
		"$Type": "Forms$LayoutGrid",
		"Rows": []any{map[string]any{
			"Columns": []any{map[string]any{"Widgets": []any{map[string]any{
				"$Type":   "Forms$DivContainer",
				"Widgets": []any{dataView("dvNested", textbox("tb"))},
			}}}},
		}},
	}
	if got := collectReported(root); len(got) != 0 {
		t.Fatalf("DataView under a grid ancestor should not be flagged, got %v", got)
	}
}

// A display-only DataView outside a grid has no label/input-width concern.
func TestWalkForUngridedDataView_DisplayOnlyIgnored(t *testing.T) {
	dv := dataView("dvDisplay", map[string]any{"$Type": "Forms$DynamicText", "Name": "t"})
	root := map[string]any{"$Type": "Forms$DivContainer", "Widgets": []any{dv}}
	if got := collectReported(root); len(got) != 0 {
		t.Fatalf("display-only DataView should not be flagged, got %v", got)
	}
}
