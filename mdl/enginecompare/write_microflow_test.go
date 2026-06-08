package enginecompare

import "testing"

// TestWriteParity_Microflow_Skeleton validates the codec-native microflow CREATE
// path for the simplest microflow (start → end, boolean return, no parameters)
// against legacy. Activity groups are added incrementally on top of this.
func TestWriteParity_Microflow_Skeleton(t *testing.T) {
	const s = "CREATE MICROFLOW MyFirstModule.MfEmpty () RETURNS BOOLEAN BEGIN RETURN true END"
	run := func(eng Engine) string {
		p := copyProject(t)
		if _, e := Run(eng, p, s); e != nil {
			t.Fatalf("%s create: %v", eng, e)
		}
		c, e := MicroflowCanonBSON(p, "MyFirstModule", "MfEmpty")
		if e != nil {
			t.Fatalf("%s canon: %v", eng, e)
		}
		return c
	}
	if leg, msd := run(Legacy), run(ModelSDK); leg != msd {
		t.Errorf("microflow skeleton divergence:\nlegacy:   %s\nmodelsdk: %s", leg, msd)
	}
}
