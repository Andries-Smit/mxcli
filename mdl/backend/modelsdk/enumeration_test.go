// SPDX-License-Identifier: Apache-2.0

package modelsdkbackend

import "testing"

// TestReadSlice_Enumerations checks the enum adapter: values are converted (for
// the Values count) and captions decode via textElementToModel. SHOW
// ENUMERATIONS is cross-checked byte-for-byte against legacy in the plan.
func TestReadSlice_Enumerations(t *testing.T) {
	b := New()
	if err := b.Connect(fixture); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = b.Disconnect() })

	enums, err := b.ListEnumerations()
	if err != nil {
		t.Fatalf("ListEnumerations: %v", err)
	}
	if len(enums) != 7 {
		t.Fatalf("ListEnumerations count = %d, want 7", len(enums))
	}
	for _, e := range enums {
		if e.Name == "Filter_Operators" {
			if len(e.Values) != 12 {
				t.Errorf("Filter_Operators value count = %d, want 12", len(e.Values))
			}
			return
		}
	}
	t.Error("Filter_Operators enumeration not found")
}
