// SPDX-License-Identifier: Apache-2.0

package rules

import "testing"

// Tests for MPR009 helpers — the rule's outer Check() depends on a
// LintContext + reader, but the structural helpers can be exercised
// directly against synthesized BSON maps.

func TestDataViewListenTarget_ListenTargetSource(t *testing.T) {
	dv := map[string]any{
		"$Type": "Forms$DataView",
		"Name":  "customerDetail",
		"DataSource": map[string]any{
			"$Type":        "Forms$ListenTargetSource",
			"ListenTarget": "customerList",
		},
	}
	if got := dataViewListenTarget(dv); got != "customerList" {
		t.Errorf("got %q, want %q", got, "customerList")
	}
}

func TestDataViewListenTarget_OtherSourceTypes(t *testing.T) {
	for name, ds := range map[string]any{
		"DataViewSource":  map[string]any{"$Type": "Forms$DataViewSource"},
		"MicroflowSource": map[string]any{"$Type": "Forms$MicroflowSource"},
		"DatabaseSource":  map[string]any{"$Type": "Forms$DatabaseSource"},
		"nil":             nil,
	} {
		dv := map[string]any{"$Type": "Forms$DataView", "DataSource": ds}
		if got := dataViewListenTarget(dv); got != "" {
			t.Errorf("%s: expected empty, got %q", name, got)
		}
	}
}

func TestDataViewListenTarget_NonDataView(t *testing.T) {
	w := map[string]any{
		"$Type": "Forms$TextBox",
		"DataSource": map[string]any{
			"$Type":        "Forms$ListenTargetSource",
			"ListenTarget": "x",
		},
	}
	if got := dataViewListenTarget(w); got != "" {
		t.Errorf("expected empty for non-DataView, got %q", got)
	}
}

func TestIsGalleryWidget(t *testing.T) {
	gallery := map[string]any{
		"$Type": "CustomWidgets$CustomWidget",
		"Type": map[string]any{
			"WidgetId": "com.mendix.widget.web.gallery.Gallery",
		},
	}
	if !isGalleryWidget(gallery) {
		t.Error("expected true for Gallery widget")
	}

	combobox := map[string]any{
		"$Type": "CustomWidgets$CustomWidget",
		"Type": map[string]any{
			"WidgetId": "com.mendix.widget.web.combobox.Combobox",
		},
	}
	if isGalleryWidget(combobox) {
		t.Error("expected false for non-Gallery custom widget")
	}

	textbox := map[string]any{"$Type": "Forms$TextBox"}
	if isGalleryWidget(textbox) {
		t.Error("expected false for non-CustomWidget")
	}
}

// Synthesize a CustomWidget BSON map with a single TypePointer-resolved
// property carrying a PrimitiveValue.
func customWidgetWith(propKey, primitiveVal string) map[string]any {
	const id = "pt-id"
	return map[string]any{
		"$Type": "CustomWidgets$CustomWidget",
		"Type": map[string]any{
			"WidgetId": "com.mendix.widget.web.gallery.Gallery",
			"ObjectType": map[string]any{
				"PropertyTypes": []any{
					map[string]any{"PropertyKey": propKey, "$ID": id},
				},
			},
		},
		"Object": map[string]any{
			"Properties": []any{
				map[string]any{
					"TypePointer": id,
					"Value": map[string]any{
						"PrimitiveValue": primitiveVal,
					},
				},
			},
		},
	}
}

func TestCustomWidgetPropertyString_Toggle(t *testing.T) {
	g := customWidgetWith("itemSelectionMode", "toggle")
	if got := customWidgetPropertyString(g, "itemSelectionMode"); got != "toggle" {
		t.Errorf("got %q, want %q", got, "toggle")
	}
}

func TestCustomWidgetPropertyString_Clear(t *testing.T) {
	g := customWidgetWith("itemSelectionMode", "clear")
	if got := customWidgetPropertyString(g, "itemSelectionMode"); got != "clear" {
		t.Errorf("got %q, want %q", got, "clear")
	}
}

func TestCustomWidgetPropertyString_MissingKeyReturnsEmpty(t *testing.T) {
	g := customWidgetWith("someOtherKey", "x")
	if got := customWidgetPropertyString(g, "itemSelectionMode"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// End-to-end-ish: walk a synthesized page BSON, drive the predicates the
// rule uses, and confirm the (broken, fixed) verdict for both shapes.
func TestGallerySelectionListener_BrokenAndFixed(t *testing.T) {
	galleryBroken := customWidgetWith("itemSelectionMode", "clear")
	galleryBroken["Name"] = "customerList"
	galleryFixed := customWidgetWith("itemSelectionMode", "toggle")
	galleryFixed["Name"] = "customerList"

	makePage := func(g map[string]any) map[string]any {
		return map[string]any{
			"FormCall": map[string]any{
				"Arguments": []any{
					map[string]any{
						"Widgets": []any{
							g,
							map[string]any{
								"$Type": "Forms$DataView",
								"Name":  "customerDetail",
								"DataSource": map[string]any{
									"$Type":        "Forms$ListenTargetSource",
									"ListenTarget": "customerList",
								},
							},
						},
					},
				},
			},
		}
	}

	for _, tc := range []struct {
		name           string
		g              map[string]any
		wantViolations bool
	}{
		{"broken (clear)", galleryBroken, true},
		{"fixed (toggle)", galleryFixed, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			byName := collectWidgetsByName(makePage(tc.g))
			dv, ok := byName["customerDetail"]
			if !ok {
				t.Fatal("did not find customerDetail in collected widgets")
			}
			target := dataViewListenTarget(dv)
			if target != "customerList" {
				t.Fatalf("unexpected target %q", target)
			}
			gal, ok := byName[target]
			if !ok {
				t.Fatal("did not find target gallery")
			}
			if !isGalleryWidget(gal) {
				t.Fatal("target not recognized as gallery")
			}
			mode := customWidgetPropertyString(gal, "itemSelectionMode")
			violation := mode != "toggle"
			if violation != tc.wantViolations {
				t.Errorf("mode=%q violation=%v, want=%v", mode, violation, tc.wantViolations)
			}
		})
	}
}
