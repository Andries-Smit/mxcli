// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"path/filepath"
	"testing"
)

// TestAddGraphAnalysis_AdditiveAndPreserving verifies that AddGraphAnalysis runs
// the graph pass on an already-built catalog (populating communities/layers from
// refs) WITHOUT touching unrelated data — the behaviour `refresh catalog
// communities` relies on so it doesn't drop source-mode FTS data.
func TestAddGraphAnalysis_AdditiveAndPreserving(t *testing.T) {
	// File-based catalog: a shared on-disk DB so the additive tx's writes are
	// visible (an in-memory catalog pools separate connections).
	cat, err := NewFromFile(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("NewFromFile: %v", err)
	}
	defer cat.Close()
	db := cat.CatalogDB()

	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("exec %q: %v", q, err)
		}
	}
	// A snapshot (AddGraphAnalysis reads its id), a structural edge, and an
	// unrelated row that must survive the pass.
	mustExec(`INSERT INTO snapshots (SnapshotId, ProjectId) VALUES ('s1','default')`)
	mustExec(`INSERT INTO refs (SourceType, SourceId, SourceName, TargetType, TargetId, TargetName, RefKind, ModuleName, ProjectId, SnapshotId)
		VALUES ('MICROFLOW','','A.Foo','MICROFLOW','','A.Bar','call','A','default','s1'),
		       ('MICROFLOW','','A.Bar','MICROFLOW','','A.Foo','call','A','default','s1')`)
	mustExec(`INSERT INTO entities_data (Id, Name, QualifiedName, ModuleName) VALUES ('e1','Keep','A.Keep','A')`)

	if err := cat.AddGraphAnalysis(1.0); err != nil {
		t.Fatalf("AddGraphAnalysis: %v", err)
	}

	count := func(table string) int {
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		return n
	}
	if count("communities_data") == 0 {
		t.Error("expected communities to be populated")
	}
	if count("graph_layers_data") == 0 {
		t.Error("expected layers to be populated")
	}
	if count("graph_centrality_data") == 0 {
		t.Error("expected centrality to be populated")
	}
	// The unrelated row must survive (additive, not a rebuild).
	if count("entities_data") != 1 {
		t.Error("AddGraphAnalysis must not touch unrelated tables")
	}

	// Idempotent: a second run replaces, not duplicates.
	if err := cat.AddGraphAnalysis(1.0); err != nil {
		t.Fatalf("second AddGraphAnalysis: %v", err)
	}
	if got := count("communities_data"); got != 2 { // A.Foo, A.Bar
		t.Errorf("expected 2 community rows after re-run, got %d", got)
	}
}
