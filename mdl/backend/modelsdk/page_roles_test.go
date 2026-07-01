// SPDX-License-Identifier: Apache-2.0

package modelsdkbackend

import "testing"

// TestPageFromGen_ReadsAllowedRoles guards SHOW ACCESS ON PAGE and the Page
// section of SHOW SECURITY MATRIX: a page's allowed module roles are BY_NAME
// references (stored under the "AllowedModuleRoles" BSON key). The adapter must
// surface them as Page.AllowedRoles, or the security-audit commands report
// "no roles" for a restricted page on the modelsdk engine (issue #722).
func TestPageFromGen_ReadsAllowedRoles(t *testing.T) {
	b := New()
	if err := b.Connect(fixture); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = b.Disconnect() })

	pages, err := b.ListPages()
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	var roles []string
	for _, pg := range pages {
		if pg.Name == "Account_Overview" {
			for _, r := range pg.AllowedRoles {
				roles = append(roles, string(r))
			}
		}
	}
	if roles == nil {
		t.Fatal("Account_Overview not found or has no AllowedRoles (page access under-reported)")
	}
	found := false
	for _, r := range roles {
		if r == "Administration.Administrator" {
			found = true
		}
	}
	if !found {
		t.Errorf("AllowedRoles = %v, want to include Administration.Administrator", roles)
	}
}
