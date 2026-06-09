// SPDX-License-Identifier: Apache-2.0

package modelsdkbackend

import (
	"fmt"

	"github.com/mendixlabs/mxcli/model"
	"github.com/mendixlabs/mxcli/modelsdk/codec"
	"github.com/mendixlabs/mxcli/modelsdk/element"
	genDm "github.com/mendixlabs/mxcli/modelsdk/gen/domainmodels"
	mmpr "github.com/mendixlabs/mxcli/modelsdk/mpr"
	"github.com/mendixlabs/mxcli/modelsdk/mprread"
	"github.com/mendixlabs/mxcli/sdk/domainmodel"
)

// MoveEnumeration reparents an enumeration unit to its (already-updated) target
// container module. Enumerations are top-level units, so this is a unit reparent.
func (b *Backend) MoveEnumeration(enum *model.Enumeration) error {
	if enum == nil {
		return fmt.Errorf("MoveEnumeration: nil enumeration")
	}
	if b.writer == nil {
		return fmt.Errorf("MoveEnumeration: not connected for writing")
	}
	return b.writer.MoveUnit(string(enum.ID), string(enum.ContainerID))
}

// MoveConstant reparents a constant unit to its target container module.
func (b *Backend) MoveConstant(c *model.Constant) error {
	if c == nil {
		return fmt.Errorf("MoveConstant: nil constant")
	}
	if b.writer == nil {
		return fmt.Errorf("MoveConstant: not connected for writing")
	}
	return b.writer.MoveUnit(string(c.ID), string(c.ContainerID))
}

// --- Guarded gaps -----------------------------------------------------------
// These operations are not yet implemented on the codec path. They are guarded
// (per ADR-0005: refuse rather than silently drop) so the modelsdk engine fails
// honestly instead of leaving a half-applied/broken model. Full implementations
// are tracked in docs/plans/2026-06-05-adopt-modelsdk-engine.md.

const errModelSDKUnsupported = "%s: not yet supported by the modelsdk engine (needs %s) — use the legacy engine"

// MoveEntity is cross-domain-model: it removes the entity from the source DM,
// adds it to the target DM, and converts dangling associations to cross-module
// associations. Pending CreateCrossAssociation + reference rewrites.
func (b *Backend) MoveEntity(entity *domainmodel.Entity, sourceDMID, targetDMID model.ID, sourceModuleName, targetModuleName string) ([]string, error) {
	return nil, fmt.Errorf(errModelSDKUnsupported, "MoveEntity", "cross-DM move + cross-association conversion")
}

// CreateCrossAssociation creates a cross-module association (ParentID by-id +
// remote child by qualified name + delete behaviors). Pending the converter.
func (b *Backend) CreateCrossAssociation(domainModelID model.ID, ca *domainmodel.CrossModuleAssociation) error {
	return fmt.Errorf(errModelSDKUnsupported, "CreateCrossAssociation", "cross-association converter")
}

// CreateViewEntitySourceDocument creates the OQL source document (a top-level
// unit) that backs a view entity. The entity's OqlViewEntitySource references it
// by qualified name (wired in entityToGen).
func (b *Backend) CreateViewEntitySourceDocument(moduleID model.ID, moduleName, docName, oqlQuery, documentation string) (model.ID, error) {
	if b.writer == nil {
		return "", fmt.Errorf("CreateViewEntitySourceDocument: not connected for writing")
	}
	docID := model.ID(mmpr.GenerateID())
	d := genDm.NewViewEntitySourceDocument()
	d.SetID(element.ID(docID))
	d.SetName(docName)
	d.SetDocumentation(documentation)
	d.SetExcluded(false)
	d.SetExportLevel("Hidden")
	d.SetOql(oqlQuery)
	contents, err := (&codec.Encoder{}).Encode(d)
	if err != nil {
		return "", fmt.Errorf("CreateViewEntitySourceDocument: encode: %w", err)
	}
	if err := b.writer.InsertUnit(string(docID), string(moduleID), "Documents", "DomainModels$ViewEntitySourceDocument", contents); err != nil {
		return "", fmt.Errorf("CreateViewEntitySourceDocument: insert: %w", err)
	}
	return docID, nil
}

// FindAllViewEntitySourceDocumentIDs returns every ViewEntitySourceDocument unit
// named docName in the given module.
func (b *Backend) FindAllViewEntitySourceDocumentIDs(moduleName, docName string) ([]model.ID, error) {
	mod, err := b.GetModuleByName(moduleName)
	if err != nil || mod == nil {
		return nil, nil
	}
	units, err := mprread.ListUnitsWithContainer[*genDm.ViewEntitySourceDocument](b.reader)
	if err != nil {
		return nil, err
	}
	var ids []model.ID
	for _, u := range units {
		if string(u.ContainerID) == string(mod.ID) && u.Element.Name() == docName {
			ids = append(ids, model.ID(u.Element.ID()))
		}
	}
	return ids, nil
}

// FindViewEntitySourceDocumentID returns the first matching source-doc ID, or "".
func (b *Backend) FindViewEntitySourceDocumentID(moduleName, docName string) (model.ID, error) {
	ids, err := b.FindAllViewEntitySourceDocumentIDs(moduleName, docName)
	if err != nil || len(ids) == 0 {
		return "", err
	}
	return ids[0], nil
}

// DeleteViewEntitySourceDocument removes a source-doc unit by ID.
func (b *Backend) DeleteViewEntitySourceDocument(id model.ID) error {
	if b.writer == nil {
		return fmt.Errorf("DeleteViewEntitySourceDocument: not connected for writing")
	}
	return b.writer.DeleteUnit(string(id))
}

// DeleteViewEntitySourceDocumentByName removes every source-doc named docName in
// the module (no-op when none exist — the executor calls this before re-creating).
func (b *Backend) DeleteViewEntitySourceDocumentByName(moduleName, docName string) error {
	ids, err := b.FindAllViewEntitySourceDocumentIDs(moduleName, docName)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := b.DeleteViewEntitySourceDocument(id); err != nil {
			return err
		}
	}
	return nil
}
