// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"

	"github.com/mendixlabs/mxcli/model"
	"github.com/mendixlabs/mxcli/sdk/microflows"
)

const microflowDocType = "Microflows$Microflow"

// CreateMicroflow creates a microflow via ped_create_document.
//
// First slice: "shell + return" microflows — name, parameters, return type, and
// a Start -> End body whose EndEvent carries the return expression (a computed
// value). Microflows with activity bodies (Create/Change/Retrieve/… objects)
// are rejected with a clear error; the 130+ activity object types are an
// iterative follow-on. See docs/03-development/PED_MCP_CAPABILITIES.md.
func (b *Backend) CreateMicroflow(mf *microflows.Microflow) error {
	mod, err := b.reader.GetModule(mf.ContainerID)
	if err != nil {
		return fmt.Errorf("resolve module for microflow %q: %w", mf.Name, err)
	}

	// The body must be only Start/End — anything else is an activity we cannot
	// map yet. Capture the return expression from the (single) EndEvent.
	var returnValue string
	endCount := 0
	if mf.ObjectCollection != nil {
		for _, o := range mf.ObjectCollection.Objects {
			switch obj := o.(type) {
			case *microflows.StartEvent:
			case *microflows.EndEvent:
				endCount++
				returnValue = obj.ReturnValue
			default:
				return fmt.Errorf("microflow %q: activity bodies are not yet supported by the MCP backend (only parameters and a return value); found %T", mf.Name, o)
			}
		}
	}
	if endCount > 1 {
		return fmt.Errorf("microflow %q: multiple return/end paths are not yet supported by the MCP backend", mf.Name)
	}

	// Build the canvas objects: parameters, then Start, then End. Parameters are
	// laid out to the left; the flow connects only Start -> End.
	objects := make([]any, 0, len(mf.Parameters)+2)
	x := 80
	for _, p := range mf.Parameters {
		typeName, entity, enumeration, err := mfDataType(p.Type)
		if err != nil {
			return fmt.Errorf("microflow %q parameter %q: %w", mf.Name, p.Name, err)
		}
		po := map[string]any{
			"$Type":               "Microflows$MicroflowParameterObject",
			"name":                p.Name,
			"type":                typeName,
			"relativeMiddlePoint": map[string]int{"x": x, "y": 200},
		}
		if entity != "" {
			po["entity"] = entity
		}
		if enumeration != "" {
			po["enumeration"] = enumeration
		}
		objects = append(objects, po)
		x += 120
	}

	startIdx := len(objects)
	objects = append(objects, map[string]any{
		"$Type":               "Microflows$StartEvent",
		"relativeMiddlePoint": map[string]int{"x": x, "y": 100},
	})
	endIdx := len(objects)
	endObj := map[string]any{
		"$Type":               "Microflows$EndEvent",
		"relativeMiddlePoint": map[string]int{"x": x + 240, "y": 100},
	}
	if returnValue != "" {
		endObj["returnValue"] = returnValue
	}
	objects = append(objects, endObj)

	flows := []any{map[string]any{
		"originId":      fmt.Sprintf("$id(/objects/%d)", startIdx),
		"destinationId": fmt.Sprintf("$id(/objects/%d)", endIdx),
	}}

	returnTypeName, rtEntity, rtEnum, err := mfDataType(mf.ReturnType)
	if err != nil {
		return fmt.Errorf("microflow %q return type: %w", mf.Name, err)
	}
	returnType := map[string]any{"type": returnTypeName}
	if rtEntity != "" {
		returnType["entity"] = rtEntity
	}
	if rtEnum != "" {
		returnType["enumeration"] = rtEnum
	}

	content := map[string]any{
		"name":               mf.Name,
		"objects":            objects,
		"flows":              flows,
		"returnType":         returnType,
		"returnVariableName": mf.ReturnVariableName,
	}

	if err := b.ensureSchema(microflowDocType); err != nil {
		return err
	}
	if err := b.pedCreateDocument(mod.Name, microflowDocType, mf.Name, content); err != nil {
		return err
	}
	if mf.ID == "" {
		mf.ID = model.ID("mcp~mf~" + mod.Name + "~" + mf.Name)
	}
	b.sessionMicroflows = append(b.sessionMicroflows, mf)
	return b.pedCheckDocument(microflowDocType, mod.Name+"."+mf.Name)
}

// ListMicroflows returns microflows from the local reader merged with those
// created over MCP this session (session entries take precedence by
// module+name) — for duplicate detection and create-then-reference in one run.
func (b *Backend) ListMicroflows() ([]*microflows.Microflow, error) {
	local, err := b.reader.ListMicroflows()
	if err != nil {
		return nil, err
	}
	if len(b.sessionMicroflows) == 0 {
		return local, nil
	}
	seen := make(map[string]bool, len(b.sessionMicroflows))
	out := make([]*microflows.Microflow, 0, len(local)+len(b.sessionMicroflows))
	for _, m := range b.sessionMicroflows {
		seen[mfKey(m)] = true
		out = append(out, m)
	}
	for _, m := range local {
		if !seen[mfKey(m)] {
			out = append(out, m)
		}
	}
	return out, nil
}

// GetMicroflow resolves by ID, preferring session-created microflows.
func (b *Backend) GetMicroflow(id model.ID) (*microflows.Microflow, error) {
	for _, m := range b.sessionMicroflows {
		if m.ID == id {
			return m, nil
		}
	}
	return b.reader.GetMicroflow(id)
}

func mfKey(m *microflows.Microflow) string {
	return string(m.ContainerID) + "." + m.Name
}

// Nanoflow + rule reads delegate to the local reader (read-only); their writes
// remain unsupported via the generated base.
func (b *Backend) ListNanoflows() ([]*microflows.Nanoflow, error) { return b.reader.ListNanoflows() }
func (b *Backend) GetNanoflow(id model.ID) (*microflows.Nanoflow, error) {
	return b.reader.GetNanoflow(id)
}
func (b *Backend) IsRule(qualifiedName string) (bool, error) { return b.reader.IsRule(qualifiedName) }

// mfDataType maps a microflow DataType onto the PED parameter/return type enum,
// returning (typeName, entityQualifiedName, enumerationQualifiedName). A nil
// DataType is Void (a microflow with no return value).
func mfDataType(dt microflows.DataType) (typeName, entity, enumeration string, err error) {
	if dt == nil {
		return "Void", "", "", nil
	}
	switch dt.GetTypeName() {
	case "Boolean", "Integer", "Decimal", "String", "DateTime":
		return dt.GetTypeName(), "", "", nil
	case "Date":
		return "DateTime", "", "", nil
	case "Object":
		return "Object", mfEntityName(dt), "", nil
	case "List":
		return "List", mfEntityName(dt), "", nil
	case "Enumeration":
		return "Enumeration", "", mfEnumName(dt), nil
	default:
		return "", "", "", fmt.Errorf("data type %q is not yet supported by the MCP backend", dt.GetTypeName())
	}
}

func mfEntityName(dt microflows.DataType) string {
	switch t := dt.(type) {
	case *microflows.ObjectType:
		return t.EntityQualifiedName
	case microflows.ObjectType:
		return t.EntityQualifiedName
	case *microflows.ListType:
		return t.EntityQualifiedName
	case microflows.ListType:
		return t.EntityQualifiedName
	}
	return ""
}

func mfEnumName(dt microflows.DataType) string {
	switch t := dt.(type) {
	case *microflows.EnumerationType:
		return t.EnumerationQualifiedName
	case microflows.EnumerationType:
		return t.EnumerationQualifiedName
	}
	return ""
}
