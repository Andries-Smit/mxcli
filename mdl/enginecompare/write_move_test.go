package enginecompare

import "testing"

// TestWriteParity_MoveEntity moves an entity (whose association to a same-module
// entity becomes a cross-module association) and checks the moved entity matches
// legacy in the target module. Cross-association correctness is covered by mx check.
func TestWriteParity_MoveEntity(t *testing.T) {
	setup := []string{
		"CREATE MODULE SrcM",
		"CREATE MODULE DstM",
		"CREATE PERSISTENT ENTITY SrcM.Parent ( Code: string(20) )",
		"CREATE PERSISTENT ENTITY SrcM.Child ( Label: string(20) )",
		"CREATE ASSOCIATION SrcM.Parent_Child FROM SrcM.Parent TO SrcM.Child TYPE reference OWNER default",
		"MOVE ENTITY SrcM.Child TO DstM",
	}
	run := func(eng Engine) string {
		p := copyProject(t)
		for _, s := range setup {
			if _, e := Run(eng, p, s); e != nil {
				t.Fatalf("%s %q: %v", eng, s, e)
			}
		}
		s, e := EntityCanonBSON(p, "DstM", "Child")
		if e != nil {
			t.Fatalf("%s canon: %v", eng, e)
		}
		return s
	}
	if leg, msd := run(Legacy), run(ModelSDK); leg != msd {
		t.Errorf("moved-entity divergence:\nlegacy:   %s\nmodelsdk: %s", leg, msd)
	}
}
