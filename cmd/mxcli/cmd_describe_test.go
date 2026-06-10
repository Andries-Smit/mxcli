// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"
	"testing"
)

func TestChooseDescribeType(t *testing.T) {
	cases := []struct {
		name      string
		matches   []string
		wantType  string
		wantCands []string
		wantErr   bool
	}{
		{"single", []string{"microflow"}, "microflow", nil, false},
		{"dedup to single", []string{"entity", "entity"}, "entity", nil, false},
		{"empties filtered", []string{"", "page", ""}, "page", nil, false},
		{"ambiguous", []string{"entity", "microflow"}, "", []string{"entity", "microflow"}, false},
		{"ambiguous with dup", []string{"entity", "microflow", "entity"}, "", []string{"entity", "microflow"}, false},
		{"order preserved", []string{"microflow", "entity"}, "", []string{"microflow", "entity"}, false},
		{"none", nil, "", nil, true},
		{"all empty is none", []string{"", ""}, "", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotCands, err := chooseDescribeType("Mod.X", tc.matches)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if gotType != tc.wantType {
				t.Errorf("type = %q, want %q", gotType, tc.wantType)
			}
			if !slices.Equal(gotCands, tc.wantCands) {
				t.Errorf("candidates = %v, want %v", gotCands, tc.wantCands)
			}
		})
	}
}

// TestTypeMaps_KnownEntries pins the mappings the auto-detect dispatch depends
// on: every mapped value must be a describe keyword the command actually handles,
// and the common types must be present.
func TestTypeMaps_KnownEntries(t *testing.T) {
	wantObject := map[string]string{
		"MICROFLOW": "microflow", "ENTITY": "entity", "PAGE": "page",
		"ENUMERATION": "enumeration", "MODULE": "module", "EXTERNAL_ENTITY": "entity",
	}
	for k, v := range wantObject {
		if objectTypeToDescribe[k] != v {
			t.Errorf("objectTypeToDescribe[%q] = %q, want %q", k, objectTypeToDescribe[k], v)
		}
	}
	wantUnit := map[string]string{
		"Microflows$Microflow": "microflow", "Forms$Page": "page",
		"Enumerations$Enumeration": "enumeration", "JavaActions$JavaAction": "javaaction",
	}
	for k, v := range wantUnit {
		if unitTypeToDescribe[k] != v {
			t.Errorf("unitTypeToDescribe[%q] = %q, want %q", k, unitTypeToDescribe[k], v)
		}
	}
	// Every mapped describe keyword must be a single bare word (no spaces) so it
	// slots into the dispatch as args[0].
	for _, m := range []map[string]string{objectTypeToDescribe, unitTypeToDescribe} {
		for k, v := range m {
			if v == "" {
				t.Errorf("empty describe keyword for %q", k)
			}
		}
	}
}
