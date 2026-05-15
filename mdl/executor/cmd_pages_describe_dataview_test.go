// SPDX-License-Identifier: Apache-2.0

package executor

import "testing"

// TestExtractDataViewDataSource_ListenTargetSource covers the bug where
// DESCRIBE dropped `DataSource: selection <widget>` for DataViews bound to a
// Gallery/ListView/DataGrid selection. Forms$ListenTargetSource was an
// unhandled $Type so extractDataViewDataSource returned nil.
func TestExtractDataViewDataSource_ListenTargetSource(t *testing.T) {
	ctx, _ := newMockCtx(t)

	raw := map[string]any{
		"$Type": "Forms$DataView",
		"Name":  "customerDetail",
		"DataSource": map[string]any{
			"$Type":        "Forms$ListenTargetSource",
			"ListenTarget": "customerList",
		},
	}

	ds := extractDataViewDataSource(ctx, raw)
	if ds == nil {
		t.Fatal("extractDataViewDataSource returned nil for Forms$ListenTargetSource")
	}
	if ds.Type != "selection" {
		t.Errorf("Type: got %q, want %q", ds.Type, "selection")
	}
	if ds.Reference != "customerList" {
		t.Errorf("Reference: got %q, want %q", ds.Reference, "customerList")
	}
}

func TestExtractDataViewDataSource_ListenTargetSourceEmptyTarget(t *testing.T) {
	ctx, _ := newMockCtx(t)

	raw := map[string]any{
		"$Type": "Forms$DataView",
		"DataSource": map[string]any{
			"$Type":        "Forms$ListenTargetSource",
			"ListenTarget": "",
		},
	}

	if ds := extractDataViewDataSource(ctx, raw); ds != nil {
		t.Errorf("expected nil for empty ListenTarget, got %+v", ds)
	}
}
