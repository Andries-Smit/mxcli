// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"strings"
	"testing"
)

func TestCapabilityTableParses(t *testing.T) {
	feats := pedCapabilityFeatures("1.0.0")
	if len(feats) == 0 {
		t.Fatal("embedded capability table parsed to zero features")
	}
	// Spot-check a known authorable and a known blocked feature at the 1.0.0 baseline.
	got := map[string]bool{}
	for _, f := range feats {
		got[f.Feature] = f.Available
	}
	if !got["Workflows"] {
		t.Error("Workflows should be authorable at baseline")
	}
	if got["Nanoflows, Java actions, Business-event services"] {
		t.Error("nanoflow/java/business-event create should be blocked at baseline")
	}
}

func TestServerVersionAtLeast(t *testing.T) {
	cases := []struct {
		have, want string
		ge         bool
	}{
		{"1.0.0", "1.0.0", true},
		{"1.2.0", "1.1.0", true},
		{"1.0.0", "1.1.0", false},
		{"2.0.0", "1.9.9", true},
		{"1.0", "1.0.1", false},
	}
	for _, c := range cases {
		if got := serverVersionAtLeast(c.have, c.want); got != c.ge {
			t.Errorf("serverVersionAtLeast(%q,%q) = %v, want %v", c.have, c.want, got, c.ge)
		}
	}
}

func TestCapabilityReport(t *testing.T) {
	// No client/server connected: the report renders from the table (and skips the
	// live tool list rather than panicking on a nil client).
	r := (&Backend{}).CapabilityReport()
	for _, want := range []string{
		"MCP backend capabilities",
		"Studio Pro MCP server : (unknown) (unknown)",
		"Concord gap-filler    : not connected",
		"✓ Workflows —",                                        // authorable, from table
		"✗ Nanoflows, Java actions, Business-event services —", // blocked, from table
		"Reads (SHOW / DESCRIBE",
		"PED_MCP_CAPABILITIES.md",
	} {
		if !strings.Contains(r, want) {
			t.Errorf("capability report missing %q in:\n%s", want, r)
		}
	}
	if strings.Contains(r, "PED tools present") {
		t.Error("no live client -> should not print a tool list")
	}
}
