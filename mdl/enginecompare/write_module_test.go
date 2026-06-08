package enginecompare

import "testing"

// TestWriteParity_CreateModule verifies the modelsdk engine can create a module
// (+ its mandatory contained units) such that subsequent statements land in it.
// The ModuleImpl unit is NOT byte-compared against legacy: legacy emits
// NewSortIndex as int64, but real Studio-Pro BSON (and the codec) use a double —
// legacy is wrong there. Instead, an entity created in the new module must match
// across engines, proving the module is functional.
func TestWriteParity_CreateModule(t *testing.T) {
	const script = "CREATE MODULE NewMod; " +
		"CREATE PERSISTENT ENTITY NewMod.Thing ( Code: string(20), Rank: integer )"
	run := func(eng Engine) string {
		p := copyProject(t)
		if _, e := Run(eng, p, script); e != nil {
			t.Fatalf("%s: %v", eng, e)
		}
		s, e := EntityCanonBSON(p, "NewMod", "Thing")
		if e != nil {
			t.Fatalf("%s canon: %v", eng, e)
		}
		return s
	}
	if leg, msd := run(Legacy), run(ModelSDK); leg != msd {
		t.Errorf("entity-in-new-module divergence:\nlegacy:   %s\nmodelsdk: %s", leg, msd)
	}
}
