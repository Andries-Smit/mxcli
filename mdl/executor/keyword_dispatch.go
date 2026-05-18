// SPDX-License-Identifier: Apache-2.0

package executor

import "strings"

// =============================================================================
// MDL keyword dispatch — native vs pluggable widgets, version-aware
// =============================================================================
//
// Some MDL widget keywords map to different Mendix widgets depending on the
// project's Mendix version. The headline case is DATAGRID:
//
//   Mendix 9.x – 10.x: native Forms$DataGrid
//   Mendix 11.0+:      pluggable com.mendix.widget.web.datagrid.Datagrid (React)
//
// Studio Pro's project upgrade does NOT auto-convert native widgets to their
// pluggable replacements. Migrated 11+ projects can have both stacks coexisting
// on the same page. The grammar therefore exposes both stacks via distinct
// keywords:
//
//   DATAGRID          — version-default (auto-picks per Mendix version)
//   LEGACYDATAGRID    — always native (deprecated on 11+, still allowed)
//
// (PLUGGABLEWIDGET 'com.mendix.widget...' name remains the explicit escape hatch.)
//
// This table is hand-maintained editorial policy data — it expresses our
// preference for which stack a generic keyword resolves to per Mendix version,
// not a Mendix-published mapping.

// keywordBindingKind names the runtime stack a binding produces.
type keywordBindingKind string

const (
	bindingKindNative    keywordBindingKind = "native"
	bindingKindPluggable keywordBindingKind = "pluggable"
)

// keywordBinding is one (version-range, target) entry for a keyword.
type keywordBinding struct {
	// MinVersion is the inclusive lower bound (e.g. "11.0.0"). Empty means
	// "all versions ≤ MaxVersion".
	MinVersion string
	// MaxVersion is the inclusive upper bound (e.g. "10.99.99"). Empty means
	// "all versions ≥ MinVersion".
	MaxVersion string
	// Kind is "native" or "pluggable".
	Kind keywordBindingKind
	// WidgetID is the pluggable widget id (only set when Kind == pluggable).
	WidgetID string
	// DeprecatedFrom marks the version (inclusive) at which this binding
	// becomes deprecated. Empty means "not deprecated". Used by mxcli check
	// --post-migration to flag legacy-stack widgets on newer projects.
	DeprecatedFrom string
}

// keywordMapping is the list of bindings for one MDL keyword.
type keywordMapping struct {
	Keyword  string
	Bindings []keywordBinding
}

// keywordDispatchTable is the editorial policy data for native-vs-pluggable
// keyword resolution. Maintained by hand; updated when a Mendix version
// promotes a new pluggable widget to be the default for a generic keyword.
//
// Today the entries are minimal: DATAGRID always resolves to pluggable
// (Datagrid 2.x has been the default since well before 11.0). LEGACYDATAGRID
// is reserved for an explicit native-stack request, tracked separately under
// Phase 2.1; the buildWidgetV3 switch returns a clear "not yet implemented"
// for it until a native builder lands.
//
// The table structure leaves room for richer dispatch rules — future
// version-aware splits, additional dual-stack widgets — without rewriting
// the lookup logic.
var keywordDispatchTable = []keywordMapping{
	{
		Keyword: "DATAGRID",
		Bindings: []keywordBinding{
			// Pluggable Datagrid 2.x has been the default since 9.18+; we
			// intentionally don't downgrade older versions to a hypothetical
			// native binding because we have no native builder to dispatch to.
			{MinVersion: "9.0.0", Kind: bindingKindPluggable, WidgetID: "com.mendix.widget.web.datagrid.Datagrid"},
		},
	},
}

// resolveKeywordBinding returns the binding for the given keyword and project
// version, or (nil, false) when the keyword has no entry in the dispatch
// table. Callers fall back to existing per-keyword handling when no binding
// is returned.
//
// Comparison is uppercase-insensitive on the keyword. Version comparison uses
// SemVer-style segments (no pre-release / build metadata handling).
func resolveKeywordBinding(keyword, version string) (*keywordBinding, bool) {
	upper := strings.ToUpper(keyword)
	for _, mapping := range keywordDispatchTable {
		if mapping.Keyword != upper {
			continue
		}
		for i := range mapping.Bindings {
			b := &mapping.Bindings[i]
			if versionInRange(version, b.MinVersion, b.MaxVersion) {
				return b, true
			}
		}
	}
	return nil, false
}

// versionInRange returns true when version is within [min, max] (inclusive).
// Empty min or max means "unbounded" on that side. Empty version returns true
// only if both min and max are empty (a wildcard binding).
func versionInRange(version, min, max string) bool {
	if version == "" {
		return min == "" && max == ""
	}
	if min != "" && compareVersion(version, min) < 0 {
		return false
	}
	if max != "" && compareVersion(version, max) > 0 {
		return false
	}
	return true
}

// compareVersion returns -1, 0, +1 by lexicographic comparison of dotted
// integer segments (e.g. "10.18.0" vs "11.0.0"). Non-numeric segments fall
// back to string comparison. Handles segment-count differences by treating
// missing segments as zero.
func compareVersion(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	n := len(aParts)
	if len(bParts) > n {
		n = len(bParts)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(aParts) {
			ai = parseIntOrZero(aParts[i])
		}
		if i < len(bParts) {
			bi = parseIntOrZero(bParts[i])
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}

// parseIntOrZero parses a version segment as a non-negative integer.
// Returns 0 for non-numeric or negative inputs (rare in real Mendix versions).
func parseIntOrZero(s string) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

// =============================================================================
// Public resolution API (consumed by inspection / DESCRIBE-side commands)
// =============================================================================
//
// Write-side routing in buildWidgetV3 stays in the switch — the existing
// hand-coded builders already produce correct BSON for each keyword. The
// dispatch table is consumed by:
//   - mxcli schema show <KEYWORD>   (future inspection command)
//   - DESCRIBE-side BSON storage type → keyword resolution
//   - check --post-migration to flag legacy-stack widgets

// KeywordResolution describes how an MDL keyword resolves for a given
// Mendix version. Stable enough for tests and inspection commands.
type KeywordResolution struct {
	Keyword        string
	Version        string
	Kind           string // "native" or "pluggable"
	WidgetID       string // pluggable widget id, empty for native
	DeprecatedFrom string // version where the binding becomes deprecated, empty otherwise
}

// ResolveKeyword returns how the given keyword resolves for the given
// Mendix version, or (nil, false) when the keyword has no dispatch entry.
func ResolveKeyword(keyword, version string) (*KeywordResolution, bool) {
	binding, ok := resolveKeywordBinding(keyword, version)
	if !ok {
		return nil, false
	}
	return &KeywordResolution{
		Keyword:        strings.ToUpper(keyword),
		Version:        version,
		Kind:           string(binding.Kind),
		WidgetID:       binding.WidgetID,
		DeprecatedFrom: binding.DeprecatedFrom,
	}, true
}

// =============================================================================
// Legacy native-widget catalog (read-side)
// =============================================================================
//
// LegacyWidget describes a native (Forms$*) widget that has been superseded by
// a pluggable replacement. Consumed by mxcli check --post-migration to scan a
// project's pages/snippets and report widgets that survived a Mendix upgrade
// untouched (Studio Pro does not auto-migrate native stacks to pluggable).

// LegacyWidget describes one deprecated native widget.
type LegacyWidget struct {
	// GoTypeName is the unqualified Go type name of the parsed widget struct
	// in sdk/pages (e.g. "DataGrid"). The scanner matches this against
	// reflect.TypeOf(w).Elem().Name() so we don't have to import pages here.
	GoTypeName string
	// BSONType is the corresponding BSON $Type, for diagnostic messages.
	BSONType string
	// DeprecatedFrom is the Mendix version (inclusive) at which the widget is
	// no longer the recommended stack.
	DeprecatedFrom string
	// Replacement is the recommended replacement (pluggable widget ID or
	// MDL keyword).
	Replacement string
	// Hint is a one-line user-facing message describing the recommended
	// migration path.
	Hint string
}

// LegacyWidgets is the editorial catalog of deprecated native widgets. Add new
// entries as Mendix promotes more pluggable replacements (e.g. Forms$Gallery
// → pluggable Gallery, Forms$DropDown → pluggable ComboBox, etc.).
//
// Only entries that can be detected via type-switching the parsed page widget
// tree belong here. Pluggable-on-pluggable upgrades (e.g. DataGrid 2.x point
// releases) are handled by widget-definition drift classification, not by
// this catalog.
var LegacyWidgets = []LegacyWidget{
	{
		GoTypeName:     "DataGrid",
		BSONType:       "Forms$DataGrid",
		DeprecatedFrom: "11.0.0",
		Replacement:    "com.mendix.widget.web.datagrid.Datagrid",
		Hint:           "migrate to pluggable Datagrid 2.x (`DATAGRID` keyword resolves to this on 11.0+)",
	},
}

// FindLegacyWidget returns the catalog entry for the given Go type name, or
// nil when the type isn't a known deprecated native widget. Matching is
// case-sensitive — Go type names are stable.
func FindLegacyWidget(goTypeName string) *LegacyWidget {
	for i := range LegacyWidgets {
		if LegacyWidgets[i].GoTypeName == goTypeName {
			return &LegacyWidgets[i]
		}
	}
	return nil
}

// IsDeprecatedOnVersion reports whether the legacy widget is deprecated at
// the given project Mendix version. Empty version (unknown) returns true so
// the scanner errs on the side of flagging.
func (lw *LegacyWidget) IsDeprecatedOnVersion(version string) bool {
	if version == "" || lw.DeprecatedFrom == "" {
		return true
	}
	return compareVersion(version, lw.DeprecatedFrom) >= 0
}
