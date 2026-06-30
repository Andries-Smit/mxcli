// SPDX-License-Identifier: Apache-2.0

package modelsdkbackend

import (
	"testing"

	"github.com/mendixlabs/mxcli/modelsdk/codec"
	"github.com/mendixlabs/mxcli/sdk/microflows"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestRetrieveSourceFromGen_SortBy guards the retrieve "sort by …" clause. The
// sort columns live in the DatabaseRetrieveSource's NewSortings child (a
// SortingsList); without reading it the clause is silently dropped.
func TestRetrieveSourceFromGen_SortBy(t *testing.T) {
	raw := mustMarshalFlow(bson.D{
		{Key: "$ID", Value: "rs-1"},
		{Key: "$Type", Value: "Microflows$DatabaseRetrieveSource"},
		{Key: "Entity", Value: "LoftManagement.Application"},
		{Key: "NewSortings", Value: bson.D{
			{Key: "$ID", Value: "sl-1"},
			{Key: "$Type", Value: "Microflows$SortingsList"},
			{Key: "Sortings", Value: bson.A{
				int32(1), // typed-array marker
				bson.D{
					{Key: "$ID", Value: "rsort-1"},
					{Key: "$Type", Value: "Microflows$RetrieveSorting"},
					{Key: "SortOrder", Value: "asc"},
					{Key: "AttributeRef", Value: bson.D{
						{Key: "$ID", Value: "ar-1"},
						{Key: "$Type", Value: "DomainModels$AttributeRef"},
						{Key: "Attribute", Value: "LoftManagement.Application.Name"},
					}},
				},
			}},
		}},
	})
	el, err := codec.NewDecoder(codec.DefaultRegistry).Decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	src, ok := retrieveSourceFromGen(el).(*microflows.DatabaseRetrieveSource)
	if !ok {
		t.Fatalf("retrieveSourceFromGen → not a DatabaseRetrieveSource")
	}
	if len(src.Sorting) != 1 {
		t.Fatalf("Sorting = %d items, want 1 (NewSortings dropped)", len(src.Sorting))
	}
	if got := src.Sorting[0]; got.AttributeQualifiedName != "LoftManagement.Application.Name" || string(got.Direction) != "asc" {
		t.Errorf("sort item = {%q, %q}, want {LoftManagement.Application.Name, asc}", got.AttributeQualifiedName, got.Direction)
	}
}
