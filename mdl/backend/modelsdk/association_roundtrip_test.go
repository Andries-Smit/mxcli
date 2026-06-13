// SPDX-License-Identifier: Apache-2.0

package modelsdkbackend

import (
	"testing"

	"github.com/mendixlabs/mxcli/sdk/domainmodel"
)

// TestUpdateDomainModel_PreservesAssociationType guards the CREATE OR MODIFY
// ASSOCIATION corruption: that path re-serializes the whole domain model via
// UpdateDomainModel→assocToGen, which reads Type/Owner off the semantic model.
// If assocFromGen drops them, every *other* association loses Type/Owner and
// Studio Pro can't load the domain model ("cannot destructure property 'child'").
func TestUpdateDomainModel_PreservesAssociationType(t *testing.T) {
	proj := copyFixture(t)
	b := New()
	if err := b.Connect(proj); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = b.Disconnect() })

	mod, err := b.GetModuleByName("MyFirstModule")
	if err != nil || mod == nil {
		t.Fatalf("GetModuleByName: %v", err)
	}
	dm, err := b.GetDomainModel(mod.ID)
	if err != nil {
		t.Fatalf("GetDomainModel: %v", err)
	}
	parent := &domainmodel.Entity{Name: "ZzParent", Persistable: true}
	child := &domainmodel.Entity{Name: "ZzChild", Persistable: true}
	if err := b.CreateEntity(dm.ID, parent); err != nil {
		t.Fatalf("CreateEntity parent: %v", err)
	}
	if err := b.CreateEntity(dm.ID, child); err != nil {
		t.Fatalf("CreateEntity child: %v", err)
	}
	if err := b.CreateAssociation(dm.ID, &domainmodel.Association{
		Name: "ZzChild_ZzParent", ParentID: child.ID, ChildID: parent.ID,
		Type: "Reference", Owner: "Default",
	}); err != nil {
		t.Fatalf("CreateAssociation: %v", err)
	}

	// Read the domain model back and re-persist it unchanged — exactly what
	// CREATE OR MODIFY ASSOCIATION does for a different association.
	dm2, err := b.GetDomainModel(mod.ID)
	if err != nil {
		t.Fatalf("GetDomainModel(2): %v", err)
	}
	var found *domainmodel.Association
	for _, a := range dm2.Associations {
		if a.Name == "ZzChild_ZzParent" {
			found = a
		}
	}
	if found == nil {
		t.Fatal("association not found on read")
	}
	if found.Type != "Reference" || found.Owner != "Default" {
		t.Fatalf("read lost Type/Owner: Type=%q Owner=%q (assocFromGen regression)", found.Type, found.Owner)
	}
	if err := b.UpdateDomainModel(dm2); err != nil {
		t.Fatalf("UpdateDomainModel: %v", err)
	}

	// Reopen: the association must still carry Type/Owner.
	b3 := New()
	if err := b3.Connect(proj); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	t.Cleanup(func() { _ = b3.Disconnect() })
	dm3, _ := b3.GetDomainModel(mod.ID)
	for _, a := range dm3.Associations {
		if a.Name == "ZzChild_ZzParent" {
			if a.Type != "Reference" || a.Owner != "Default" {
				t.Fatalf("UpdateDomainModel wiped Type/Owner: Type=%q Owner=%q", a.Type, a.Owner)
			}
			return
		}
	}
	t.Fatal("association missing after UpdateDomainModel")
}
