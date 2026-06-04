// SPDX-License-Identifier: Apache-2.0

package executor

import "testing"

// TestMdlIdent covers issue #619: DESCRIBE must quote identifiers that collide
// with a reserved keyword so the output re-parses, while leaving ordinary names
// (and lookalikes such as "Dot") bare.
func TestMdlIdent(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		// Reserved keywords → quoted.
		{"List", `"List"`},
		{"Column", `"Column"`},
		{"Template", `"Template"`},
		{"Attribute", `"Attribute"`},
		// Common datagrid column names that collide with keywords (issue #638).
		{"Title", `"Title"`},
		{"Description", `"Description"`},
		// Case-insensitive keywords are still keywords.
		{"list", `"list"`},
		// Ordinary identifiers → bare.
		{"ctnMain", "ctnMain"},
		{"txtName", "txtName"},
		{"_private", "_private"},
		{"Widget1", "Widget1"},
		// "Dot" must NOT be confused with the DOT punctuation token.
		{"Dot", "Dot"},
		{"Arrow", "Arrow"},
		// Not a valid bare identifier (space, leading digit) → quoted.
		{"my widget", `"my widget"`},
		{"1st", `"1st"`},
		// Empty stays empty (unnamed widgets).
		{"", ""},
	}

	for _, tc := range cases {
		if got := mdlIdent(tc.name); got != tc.want {
			t.Errorf("mdlIdent(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
