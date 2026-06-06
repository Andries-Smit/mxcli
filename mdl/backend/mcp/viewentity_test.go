// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mendixlabs/mxcli/sdk/domainmodel"
)

func TestBuildEntityValue_ViewEntity(t *testing.T) {
	b := &Backend{}
	e := &domainmodel.Entity{
		Name:              "LocView",
		Persistable:       true,
		Source:            "DomainModels$OqlViewEntitySource",
		SourceDocumentRef: "MyFirstModule.LocView",
		Attributes: []*domainmodel.Attribute{{
			Name:  "LocName",
			Type:  &domainmodel.StringAttributeType{Length: 200},
			Value: &domainmodel.AttributeValue{ViewReference: "LocName"},
		}},
	}
	v, err := b.buildEntityValue(e)
	if err != nil {
		t.Fatalf("buildEntityValue: %v", err)
	}
	raw, _ := json.Marshal(v)
	for _, want := range []string{
		`"$Type":"DomainModels$OqlViewEntitySource"`,
		`"sourceDocument":"MyFirstModule.LocView"`,
		`"$Type":"DomainModels$OqlViewValue"`,
		`"reference":"LocName"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("view entity value missing %s: %s", want, raw)
		}
	}
}

func TestBuildEntityValue_ViewEntity_RequiresSourceRef(t *testing.T) {
	b := &Backend{}
	e := &domainmodel.Entity{
		Name:        "Bad",
		Persistable: true,
		Source:      "DomainModels$OqlViewEntitySource",
		// SourceDocumentRef intentionally empty
	}
	if _, err := b.buildEntityValue(e); err == nil {
		t.Fatal("expected error when a view entity has no source document reference")
	}
}

func TestCreateViewEntitySourceDocument_Choreography(t *testing.T) {
	f := newFakePED(t, func(string, map[string]any) (string, bool) { return "SUCCESS", false })
	b := &Backend{
		client:        f.connectClient(t),
		dirty:         map[string]bool{},
		schemaFetched: map[string]bool{},
	}

	id, err := b.CreateViewEntitySourceDocument("m1", "MyFirstModule", "LocView", "select 1 as X", "")
	if err != nil {
		t.Fatalf("CreateViewEntitySourceDocument: %v", err)
	}
	if id == "" {
		t.Error("expected a non-empty source document id")
	}

	create, ok := f.callByName("ped_create_document")
	if !ok {
		t.Fatal("ped_create_document not called")
	}
	craw, _ := json.Marshal(create.Args["documents"])
	if !strings.Contains(string(craw), `"documentType":"DomainModels$ViewEntitySourceDocument"`) ||
		!strings.Contains(string(craw), `"documentName":"LocView"`) {
		t.Errorf("create-document args wrong: %s", craw)
	}

	update, ok := f.callByName("ped_update_document")
	if !ok {
		t.Fatal("ped_update_document (set /oql) not called")
	}
	uraw, _ := json.Marshal(update.Args["operations"])
	if !strings.Contains(string(uraw), `"path":"/oql"`) || !strings.Contains(string(uraw), `"select 1 as X"`) {
		t.Errorf("set /oql op wrong: %s", uraw)
	}
	if update.Args["documentName"] != "MyFirstModule.LocView" {
		t.Errorf("oql set targeted wrong doc: %v", update.Args["documentName"])
	}
}

func TestDeleteViewEntitySourceDocumentByName_NoOp(t *testing.T) {
	b := &Backend{}
	if err := b.DeleteViewEntitySourceDocumentByName("MyFirstModule", "LocView"); err != nil {
		t.Fatalf("should be a no-op, got: %v", err)
	}
}
