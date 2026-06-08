package enginecompare

import "testing"

// TestWriteParity_DropAssociation verifies DROP ASSOCIATION removes the association
// in both engines (the kept entities/associations must still match).
func TestWriteParity_DropAssociation(t *testing.T) {
	setup := []string{
		"CREATE PERSISTENT ENTITY MyFirstModule.DA_A ( Code: string(20) )",
		"CREATE PERSISTENT ENTITY MyFirstModule.DA_B ( Label: string(20) )",
		"CREATE ASSOCIATION MyFirstModule.DA_A_B FROM MyFirstModule.DA_A TO MyFirstModule.DA_B TYPE reference OWNER default",
		"DROP ASSOCIATION MyFirstModule.DA_A_B",
	}
	run := func(eng Engine) string {
		p := copyProject(t)
		for _, s := range setup {
			if _, e := Run(eng, p, s); e != nil {
				t.Fatalf("%s %q: %v", eng, s, e)
			}
		}
		s, e := EntityCanonBSON(p, "MyFirstModule", "DA_A")
		if e != nil {
			t.Fatalf("%s canon: %v", eng, e)
		}
		return s
	}
	if leg, msd := run(Legacy), run(ModelSDK); leg != msd {
		t.Errorf("drop-association divergence:\nlegacy:   %s\nmodelsdk: %s", leg, msd)
	}
}
