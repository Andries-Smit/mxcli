// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentComposeTemplateVersion(t *testing.T) {
	// The embedded template must carry a version stamp so staleness detection
	// works. If this fails, the "# mxcli-template-version: N" line was dropped.
	if got := currentComposeTemplateVersion(); got < 2 {
		t.Fatalf("embedded compose template version = %d, want >= 2 (stamp missing?)", got)
	}
}

func TestParseTemplateVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"stamped", "# mxcli-template-version: 2\nservices:\n", 2},
		{"stamped with extra spaces", "#   mxcli-template-version:   7\n", 7},
		{"no stamp (pre-versioning)", "services:\n  mendix:\n", 0},
		{"non-numeric", "# mxcli-template-version: abc\n", 0},
		{"empty", "", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseTemplateVersion([]byte(c.in)); got != c.want {
				t.Errorf("parseTemplateVersion(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestComposeFileVersion(t *testing.T) {
	dir := t.TempDir()

	// Missing file -> 0.
	if got := composeFileVersion(filepath.Join(dir, "nope.yml")); got != 0 {
		t.Errorf("missing file version = %d, want 0", got)
	}

	// Unstamped (pre-versioning) file -> 0.
	old := filepath.Join(dir, "old.yml")
	if err := os.WriteFile(old, []byte("services:\n  mendix:\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := composeFileVersion(old); got != 0 {
		t.Errorf("unstamped file version = %d, want 0", got)
	}

	// Stamped file -> stamped value.
	stamped := filepath.Join(dir, "new.yml")
	if err := os.WriteFile(stamped, []byte("# mxcli-template-version: 5\nservices:\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := composeFileVersion(stamped); got != 5 {
		t.Errorf("stamped file version = %d, want 5", got)
	}
}
