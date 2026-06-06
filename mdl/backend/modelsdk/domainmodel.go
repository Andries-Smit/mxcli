// SPDX-License-Identifier: Apache-2.0

package modelsdkbackend

import (
	genDm "github.com/mendixlabs/mxcli/modelsdk/gen/domainmodels"
	"github.com/mendixlabs/mxcli/modelsdk/mprread"

	"github.com/mendixlabs/mxcli/model"
	"github.com/mendixlabs/mxcli/sdk/domainmodel"
)

// This file is the gen→domainmodel READ adapter. engalar's fork changed the
// DomainModelBackend interface to traffic in *genDm types and deleted
// sdk/domainmodel, so there is no converter to port — keeping main's executor
// and domainmodel types canonical means we own this translation. Phase 1 covers
// the breadth SHOW ENTITIES needs (names, persistability, generalization, and
// faithful member counts); full attribute-type / association-detail fidelity
// (DESCRIBE level) is a later phase.

// ListDomainModels reads every domain model through the codec engine.
func (b *Backend) ListDomainModels() ([]*domainmodel.DomainModel, error) {
	units, err := mprread.ListUnitsWithContainer[*genDm.DomainModel](b.reader)
	if err != nil {
		return nil, err
	}
	out := make([]*domainmodel.DomainModel, 0, len(units))
	for _, u := range units {
		out = append(out, domainModelFromGen(u.Element, u.ContainerID))
	}
	return out, nil
}

// GetDomainModel returns the domain model whose container is moduleID.
func (b *Backend) GetDomainModel(moduleID model.ID) (*domainmodel.DomainModel, error) {
	units, err := mprread.ListUnitsWithContainer[*genDm.DomainModel](b.reader)
	if err != nil {
		return nil, err
	}
	for _, u := range units {
		if u.ContainerID == moduleID {
			return domainModelFromGen(u.Element, u.ContainerID), nil
		}
	}
	return nil, nil
}

func domainModelFromGen(dm *genDm.DomainModel, containerID model.ID) *domainmodel.DomainModel {
	out := &domainmodel.DomainModel{ContainerID: containerID}
	out.ID = model.ID(dm.ID())
	for _, el := range dm.EntitiesItems() {
		if e, ok := el.(*genDm.Entity); ok {
			out.Entities = append(out.Entities, entityFromGen(e))
		}
	}
	for _, el := range dm.AssociationsItems() {
		if a, ok := el.(*genDm.Association); ok {
			out.Associations = append(out.Associations, assocFromGen(a))
		}
	}
	return out
}

func entityFromGen(e *genDm.Entity) *domainmodel.Entity {
	out := &domainmodel.Entity{
		Name:          e.Name(),
		Documentation: e.Documentation(),
		Persistable:   true, // default; NoGeneralization overrides below
	}
	out.ID = model.ID(e.ID())

	// Generalization element is either NoGeneralization (carries persistability
	// + system-attribute flags) or Generalization (extends a parent entity).
	switch g := e.Generalization().(type) {
	case *genDm.NoGeneralization:
		out.Persistable = g.Persistable()
		out.HasOwner = g.HasOwner()
		out.HasChangedBy = g.HasChangedBy()
		out.HasChangedDate = g.HasChangedDate()
		out.HasCreatedDate = g.HasCreatedDate()
	case *genDm.Generalization:
		out.GeneralizationRef = g.GeneralizationQualifiedName()
		// Persistability is inherited from the parent chain; default true
		// matches legacy (sdk/mpr parser_domainmodel.go).
	}

	for _, el := range e.AttributesItems() {
		if a, ok := el.(*genDm.Attribute); ok {
			attr := &domainmodel.Attribute{Name: a.Name()}
			attr.ID = model.ID(a.ID())
			out.Attributes = append(out.Attributes, attr)
		}
	}
	for _, el := range e.AccessRulesItems() {
		ar := &domainmodel.AccessRule{}
		ar.ID = model.ID(el.ID())
		out.AccessRules = append(out.AccessRules, ar)
	}
	for _, el := range e.IndexesItems() {
		ix := &domainmodel.Index{}
		ix.ID = model.ID(el.ID())
		out.Indexes = append(out.Indexes, ix)
	}
	for _, el := range e.ValidationRulesItems() {
		vr := &domainmodel.ValidationRule{}
		vr.ID = model.ID(el.ID())
		out.ValidationRules = append(out.ValidationRules, vr)
	}
	for _, el := range e.EventHandlersItems() {
		eh := &domainmodel.EventHandler{}
		eh.ID = model.ID(el.ID())
		out.EventHandlers = append(out.EventHandlers, eh)
	}
	return out
}

func assocFromGen(a *genDm.Association) *domainmodel.Association {
	out := &domainmodel.Association{
		Name:     a.Name(),
		ParentID: model.ID(a.ParentRefID()), // FROM entity (owns the FK)
		ChildID:  model.ID(a.ChildRefID()),  // TO entity
	}
	out.ID = model.ID(a.ID())
	return out
}
