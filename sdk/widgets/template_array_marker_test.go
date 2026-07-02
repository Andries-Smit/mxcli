// SPDX-License-Identifier: Apache-2.0

// Guard against markerless empty arrays in embedded widget templates.
//
// Mendix serializes every model list as a BSON array whose FIRST element is a
// type-discriminator marker int (e.g. Texts$Text.Items -> [3], ClientTemplate.
// Parameters -> [2], Widgets/Objects -> [2]). A hand-authored template that
// writes an empty list as a bare "[]" (no marker) produces an array that older
// Mendix tolerates but 11.12's StreamingBsonUnitReader mis-parses, aborting the
// whole project load with:
//
//	System.InvalidOperationException: Type ...CustomWidgets.WidgetProperty does
//	not contain a constructor with a parameter of type ...CustomWidgets.WidgetValue.
//
// This exact defect shipped in datagrid-number-filter.json (its placeholder /
// screen-reader ClientTemplate blocks had "Items": [] and "Parameters": []),
// corrupting any .mpr that used a NUMBERFILTER — it passed `mxcli check` but
// failed MxBuild/Studio Pro load. See fix-issue.md. The invariant below holds
// for every other template, so we pin it: no embedded template may contain a
// bare, markerless empty array.

package widgets

import (
	"encoding/json"
	"testing"
)

// findMarkerlessEmptyArrays walks a decoded template JSON value and returns the
// dotted paths of every empty array ([]). In these templates an array is always
// a Mendix list, which must carry a leading marker int — so an empty array is
// unconditionally malformed.
func findMarkerlessEmptyArrays(v any, path string, out *[]string) {
	switch n := v.(type) {
	case map[string]any:
		for k, cv := range n {
			findMarkerlessEmptyArrays(cv, path+"/"+k, out)
		}
	case []any:
		if len(n) == 0 {
			*out = append(*out, path)
			return
		}
		for i, e := range n {
			findMarkerlessEmptyArrays(e, path+"/"+itoa(i), out)
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// TestTemplates_NoMarkerlessEmptyArrays asserts that no embedded widget template
// contains a bare "[]" array — every Mendix list must serialize with its marker
// int, or 11.12 fails to load the project (see file header).
func TestTemplates_NoMarkerlessEmptyArrays(t *testing.T) {
	entries, err := templateFS.ReadDir("templates/mendix-11.6")
	if err != nil {
		t.Fatalf("read template dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		data, err := templateFS.ReadFile("templates/mendix-11.6/" + name)
		if err != nil {
			t.Fatalf("%s: read: %v", name, err)
		}
		var doc any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		var bad []string
		findMarkerlessEmptyArrays(doc, "", &bad)
		for _, p := range bad {
			t.Errorf("%s: markerless empty array at %s — a Mendix list must carry a leading marker int (e.g. Items->[3], Parameters->[2]); a bare [] corrupts the .mpr on Mendix 11.12 load", name, p)
		}
	}
}
