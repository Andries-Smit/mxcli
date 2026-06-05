// SPDX-License-Identifier: Apache-2.0

// Package marketplace provides a typed REST client for
// marketplace-api.mendix.com. See docs/11-proposals/PROPOSAL_marketplace_modules.md
// for context and the discovery spike that pinned down these shapes.
package marketplace

import "time"

// BaseURL is the marketplace REST API hostname. Exposed as a var so tests
// can redirect to an httptest.Server.
var BaseURL = "https://marketplace-api.mendix.com"

// Content is a marketplace item (module, widget, theme, etc.).
//
// Only fields confirmed by the 2026-04 discovery spike are typed; the API
// may return additional fields that are silently dropped during decoding.
type Content struct {
	ContentID       int        `json:"contentId"`
	Publisher       string     `json:"publisher"`
	Type            string     `json:"type"` // "Module", "Widget", "Theme", "Starter App", ...
	Categories      []Category `json:"categories"`
	SupportCategory string     `json:"supportCategory"` // "Platform", "Community", "Deprecated", ...
	LicenseURL      string     `json:"licenseUrl,omitempty"`
	IsPrivate       bool       `json:"isPrivate"`
	LatestVersion   *Version   `json:"latestVersion,omitempty"`
}

// Category is a marketplace content category tag.
type Category struct {
	Name string `json:"name"`
}

// Version describes a single published version of a Content item.
type Version struct {
	Name                      string    `json:"name"`
	VersionID                 string    `json:"versionId"` // UUID
	VersionNumber             string    `json:"versionNumber"`
	MinSupportedMendixVersion string    `json:"minSupportedMendixVersion"`
	PublicationDate           time.Time `json:"publicationDate"`
	ReleaseNotes              string    `json:"releaseNotes,omitempty"` // HTML
	VersionType               string    `json:"versionType,omitempty"`  // "Regular", ...
	// DownloadURL is the API endpoint that 303-redirects to the .mpk on the
	// public CDN; requires MxToken auth on the first hop. Empty for versions the
	// API exposes no download for. Added 2026-06; see the marketplace download
	// reference memory for the verified flow.
	DownloadURL string `json:"downloadUrl,omitempty"`
}

// ContentList is the list shape returned by search/list endpoints.
type ContentList struct {
	Items []Content `json:"items"`
}

// VersionList is the list shape returned by the versions endpoint.
type VersionList struct {
	Items []Version `json:"items"`
}
