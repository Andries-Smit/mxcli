// SPDX-License-Identifier: Apache-2.0

package modelsdkbackend

import "testing"

// TestReadSlice_Entities exercises the gen->domainmodel adapter against the
// fixture: it reads the Administration domain model and checks that the
// converted Account entity carries the persistability, generalization, and
// member counts the SHOW ENTITIES renderer relies on. These values are
// cross-checked end-to-end against the legacy engine in the plan's validation.
func TestReadSlice_Entities(t *testing.T) {
	b := New()
	if err := b.Connect(fixture); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = b.Disconnect() })

	mod, err := b.GetModuleByName("Administration")
	if err != nil || mod == nil {
		t.Fatalf("GetModuleByName(Administration) = %v, %v", mod, err)
	}

	dm, err := b.GetDomainModel(mod.ID)
	if err != nil {
		t.Fatalf("GetDomainModel: %v", err)
	}
	if dm == nil {
		t.Fatal("GetDomainModel returned nil for Administration")
	}
	if dm.ContainerID != mod.ID {
		t.Errorf("domain model ContainerID = %q, want module ID %q", dm.ContainerID, mod.ID)
	}

	var account *struct {
		persistable bool
		extends     string
		attrs       int
		access      int
	}
	for _, e := range dm.Entities {
		if e.Name == "Account" {
			account = &struct {
				persistable bool
				extends     string
				attrs       int
				access      int
			}{e.Persistable, e.GeneralizationRef, len(e.Attributes), len(e.AccessRules)}
		}
	}
	if account == nil {
		t.Fatalf("Account entity not found among %d entities", len(dm.Entities))
	}
	if !account.persistable {
		t.Error("Account should be persistable")
	}
	if account.extends != "System.User" {
		t.Errorf("Account extends = %q, want System.User", account.extends)
	}
	if account.attrs != 3 {
		t.Errorf("Account attribute count = %d, want 3", account.attrs)
	}
	if account.access != 3 {
		t.Errorf("Account access-rule count = %d, want 3", account.access)
	}

	// ListDomainModels covers all real modules (System is injected by the legacy
	// backend, not present as units — see the known-gap note in the plan).
	all, err := b.ListDomainModels()
	if err != nil {
		t.Fatalf("ListDomainModels: %v", err)
	}
	if len(all) == 0 {
		t.Fatal("ListDomainModels returned none")
	}
}
