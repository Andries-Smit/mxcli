// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"embed"
	"os"
	"regexp"
	"strconv"
)

//go:embed templates/*
var templatesFS embed.FS

// templateVersionRegex extracts the "# mxcli-template-version: N" stamp that
// the generated docker-compose.yml carries so mxcli can detect when a project's
// compose file predates the current template (and thus misses template fixes).
var templateVersionRegex = regexp.MustCompile(`(?m)^#\s*mxcli-template-version:\s*(\d+)`)

// currentComposeTemplateVersion returns the version stamped in the embedded
// docker-compose template (the single source of truth). Returns 0 if the stamp
// is missing, which should never happen for a released binary.
func currentComposeTemplateVersion() int {
	data, err := templatesFS.ReadFile("templates/docker-compose.yml")
	if err != nil {
		return 0
	}
	return parseTemplateVersion(data)
}

// composeFileVersion returns the template version stamped in a generated
// docker-compose.yml on disk. Returns 0 when the file has no stamp (i.e. it was
// generated before versioning was introduced) or cannot be read.
func composeFileVersion(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return parseTemplateVersion(data)
}

func parseTemplateVersion(data []byte) int {
	m := templateVersionRegex.FindSubmatch(data)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return 0
	}
	return n
}
