// SPDX-License-Identifier: Apache-2.0

package modelsdkbackend

import (
	"fmt"

	"github.com/mendixlabs/mxcli/model"
	genDm "github.com/mendixlabs/mxcli/modelsdk/gen/domainmodels"
)

// ReconcileMemberAccesses brings every populated access rule in a domain model
// into sync with its entity's current members: it adds a MemberAccess for each
// attribute, each FROM-side association (regular + cross), and each implicit
// system association (System.owner / System.changedBy); removes stale entries
// for members that no longer exist; and downgrades write rights on calculated
// attributes (CE6592). It mirrors the legacy writer's reconcile and is invoked
// by the executor's finalize step after every program run.
//
// Rules with no MemberAccesses yet (a fresh, empty rule) are left untouched —
// matching legacy; those are populated at create time by the inline sync in
// entityToGen, not retro-actively here. Members are added in definition order
// so the serialized output is deterministic.
func (b *Backend) ReconcileMemberAccesses(unitID model.ID, moduleName string) (int, error) {
	if b.writer == nil {
		return 0, fmt.Errorf("ReconcileMemberAccesses: not connected for writing")
	}
	dm, err := b.loadDomainModelGen(unitID)
	if err != nil {
		return 0, err
	}

	modified := 0
	for _, el := range dm.EntitiesItems() {
		ent, ok := el.(*genDm.Entity)
		if !ok {
			continue
		}
		entityID := string(ent.ID())
		entityName := ent.Name()
		if entityName == "" {
			continue
		}

		// Attributes (in order) with calculated flags.
		type attrInfo struct {
			qn   string
			calc bool
		}
		var attrs []attrInfo
		attrSet := map[string]bool{}
		calcSet := map[string]bool{}
		for _, ae := range ent.AttributesItems() {
			a, ok := ae.(*genDm.Attribute)
			if !ok {
				continue
			}
			qn := moduleName + "." + entityName + "." + a.Name()
			_, isCalc := a.Value().(*genDm.CalculatedValue)
			attrs = append(attrs, attrInfo{qn, isCalc})
			attrSet[qn] = true
			if isCalc {
				calcSet[qn] = true
			}
		}

		// FROM-side associations (ParentPointer == this entity), regular + cross.
		var assocQNs []string
		assocSet := map[string]bool{}
		addAssoc := func(name string) {
			if name == "" {
				return
			}
			qn := moduleName + "." + name
			if !assocSet[qn] {
				assocSet[qn] = true
				assocQNs = append(assocQNs, qn)
			}
		}
		for _, ae := range dm.AssociationsItems() {
			if a, ok := ae.(*genDm.Association); ok && string(a.ParentRefID()) == entityID {
				addAssoc(a.Name())
			}
		}
		for _, ce := range dm.CrossAssociationsItems() {
			if ca, ok := ce.(*genDm.CrossAssociation); ok && string(ca.ParentRefID()) == entityID {
				addAssoc(ca.Name())
			}
		}

		// Implicit system associations from NoGeneralization flags.
		var sysRefs []string
		sysSet := map[string]bool{}
		if ng, ok := ent.Generalization().(*genDm.NoGeneralization); ok {
			if ng.HasOwner() {
				sysRefs = append(sysRefs, "System.owner")
				sysSet["System.owner"] = true
			}
			if ng.HasChangedBy() {
				sysRefs = append(sysRefs, "System.changedBy")
				sysSet["System.changedBy"] = true
			}
		}

		for _, re := range ent.AccessRulesItems() {
			rule, ok := re.(*genDm.AccessRule)
			if !ok {
				continue
			}
			mas := rule.MemberAccessesItems()
			if len(mas) == 0 {
				continue // empty rule: populated at create time, not here
			}
			defRights := rule.DefaultMemberAccessRights()
			if defRights == "" {
				defRights = "ReadWrite"
			}

			covAttr := map[string]bool{}
			covAssoc := map[string]bool{}
			covSys := map[string]bool{}
			changed := false

			// Walk existing entries back-to-front: drop stale, downgrade calc.
			// Removing at index i leaves lower indices valid, so backward is safe.
			for i := len(mas) - 1; i >= 0; i-- {
				ma, ok := mas[i].(*genDm.MemberAccess)
				if !ok {
					continue
				}
				switch attrRef, assocRef := ma.AttributeQualifiedName(), ma.AssociationQualifiedName(); {
				case attrRef != "":
					if attrSet[attrRef] {
						covAttr[attrRef] = true
						if calcSet[attrRef] {
							if r := ma.AccessRights(); r == "ReadWrite" || r == "WriteOnly" {
								ma.SetAccessRights("ReadOnly")
								changed = true
							}
						}
					} else {
						rule.RemoveMemberAccesses(i)
						changed = true
					}
				case assocRef != "":
					switch {
					case sysSet[assocRef]:
						covSys[assocRef] = true
					case assocSet[assocRef]:
						covAssoc[assocRef] = true
					default:
						rule.RemoveMemberAccesses(i)
						changed = true
					}
				}
			}

			// Add missing members, in definition order.
			for _, ai := range attrs {
				if covAttr[ai.qn] {
					continue
				}
				rights := defRights
				if ai.calc && (rights == "ReadWrite" || rights == "WriteOnly") {
					rights = "ReadOnly"
				}
				rule.AddMemberAccesses(newMemberAccess(rights, ai.qn, true))
				changed = true
			}
			for _, qn := range assocQNs {
				if !covAssoc[qn] {
					rule.AddMemberAccesses(newMemberAccess(defRights, qn, false))
					changed = true
				}
			}
			for _, ref := range sysRefs {
				if !covSys[ref] {
					rule.AddMemberAccesses(newMemberAccess(defRights, ref, false))
					changed = true
				}
			}

			if changed {
				modified++
			}
		}
	}

	if modified > 0 {
		if err := b.persistDM(unitID, dm); err != nil {
			return 0, err
		}
	}
	return modified, nil
}

// newMemberAccess builds a fresh MemberAccess (with its own $ID) for either an
// attribute (isAttr=true) or an association reference.
func newMemberAccess(rights, qualifiedName string, isAttr bool) *genDm.MemberAccess {
	ma := genDm.NewMemberAccess()
	ma.SetAccessRights(rights)
	if isAttr {
		ma.SetAttributeQualifiedName(qualifiedName)
	} else {
		ma.SetAssociationQualifiedName(qualifiedName)
	}
	assignID(ma)
	return ma
}
