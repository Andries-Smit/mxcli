// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// capabilities.yaml is the version-keyed table half of the capability model
// (ADR-0004): the non-probeable facts (create whitelist, behavioral quirks). Tool
// presence comes from a live tools/list probe, merged in by (*Backend).capabilities.
//
//go:embed capabilities.yaml
var capabilityTableYAML []byte

// Capability is one authorable/blocked feature for a given server version.
type Capability struct {
	Feature   string
	Available bool
	Note      string
}

// Capabilities is the effective capability set for a connected server: the
// version-keyed table merged with the live server identity and tool probe. The
// agent-facing report and (in slice 3) the backend's authoring gates read from it,
// so they cannot drift.
type Capabilities struct {
	ServerName       string
	ServerVersion    string
	ConcordConnected bool
	Tools            []string
	Features         []Capability
}

type capabilityTable struct {
	BaselineServerVersion string `yaml:"baseline_server_version"`
	Features              []struct {
		Feature        string `yaml:"feature"`
		Available      bool   `yaml:"available"`
		AvailableSince string `yaml:"available_since"`
		Note           string `yaml:"note"`
	} `yaml:"features"`
}

// pedCapabilityFeatures resolves the feature capabilities for a connected MCP
// server version. A feature blocked at baseline becomes available once the server
// reaches its `available_since` — so lifting a PED limit is a one-line table edit.
func pedCapabilityFeatures(serverVersion string) []Capability {
	var t capabilityTable
	// Embedded + validated by TestCapabilityTableParses; a parse failure here would
	// be a build-time content bug, so degrade to empty rather than panic.
	_ = yaml.Unmarshal(capabilityTableYAML, &t)
	out := make([]Capability, 0, len(t.Features))
	for _, f := range t.Features {
		available := f.Available
		if !available && f.AvailableSince != "" && serverVersionAtLeast(serverVersion, f.AvailableSince) {
			available = true
		}
		out = append(out, Capability{Feature: f.Feature, Available: available, Note: f.Note})
	}
	return out
}

// capabilities builds the effective capability set: the version-keyed table for the
// connected server version, plus live identity/Concord/tools.
func (b *Backend) capabilities() Capabilities {
	caps := Capabilities{
		ServerName:       b.server.Name,
		ServerVersion:    b.server.Version,
		ConcordConnected: b.concord != nil,
		Features:         pedCapabilityFeatures(b.server.Version),
	}
	if b.client != nil {
		if tools, err := b.client.ListTools(); err == nil {
			sort.Strings(tools)
			caps.Tools = tools
		}
	}
	return caps
}

// CapabilityReport renders a human-readable summary of what the MCP backend can
// author against the connected Studio Pro server — so an agent can check, before
// generating MDL, which operations are possible against this version. It is
// generated entirely from (*Backend).capabilities (no hardcoded lists).
func (b *Backend) CapabilityReport() string {
	caps := b.capabilities()
	var sb strings.Builder
	sb.WriteString("MCP backend capabilities\n")
	sb.WriteString("========================\n")
	fmt.Fprintf(&sb, "Studio Pro MCP server : %s %s\n", orUnknown(caps.ServerName), orUnknown(caps.ServerVersion))
	concord := "not connected — DROP of standalone docs (enum/microflow/page/…) is unavailable"
	if caps.ConcordConnected {
		concord = "connected"
	}
	fmt.Fprintf(&sb, "Concord gap-filler    : %s\n\n", concord)

	sb.WriteString("Authorable over MCP:\n")
	for _, c := range caps.Features {
		if c.Available {
			fmt.Fprintf(&sb, "  ✓ %s — %s\n", c.Feature, c.Note)
		}
	}
	sb.WriteString("\nNot authorable (PED limits this version):\n")
	for _, c := range caps.Features {
		if !c.Available {
			fmt.Fprintf(&sb, "  ✗ %s — %s\n", c.Feature, c.Note)
		}
	}
	sb.WriteString("\nReads (SHOW / DESCRIBE of any document type): always available from the local .mpr.\n")

	if len(caps.Tools) > 0 {
		fmt.Fprintf(&sb, "\nPED tools present (%d): %s\n", len(caps.Tools), strings.Join(caps.Tools, ", "))
	}
	sb.WriteString("\nDetail & per-version onboarding: docs/03-development/PED_MCP_CAPABILITIES.md\n")
	return sb.String()
}

func orUnknown(s string) string {
	if s == "" {
		return "(unknown)"
	}
	return s
}

// serverVersionAtLeast reports whether have >= want for dotted numeric versions
// (e.g. "1.2.0" >= "1.1.0"). Non-numeric segments compare as 0.
func serverVersionAtLeast(have, want string) bool {
	h, w := splitVersion(have), splitVersion(want)
	for i := 0; i < len(h) || i < len(w); i++ {
		var hv, wv int
		if i < len(h) {
			hv = h[i]
		}
		if i < len(w) {
			wv = w[i]
		}
		if hv != wv {
			return hv > wv
		}
	}
	return true // equal
}

func splitVersion(v string) []int {
	parts := strings.Split(v, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, _ := strconv.Atoi(strings.TrimFunc(p, func(r rune) bool { return r < '0' || r > '9' }))
		out[i] = n
	}
	return out
}
