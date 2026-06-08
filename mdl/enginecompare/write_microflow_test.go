package enginecompare

import "testing"

// TestWriteParity_Microflow_ObjectOps validates the object-operations group
// (create/change/commit/delete/rollback) against legacy. The entity is set up via
// legacy on both copies; only the microflow CREATE differs by engine.
func TestWriteParity_Microflow_ObjectOps(t *testing.T) {
	const setup = "CREATE PERSISTENT ENTITY MyFirstModule.Thing ( Name: string(100), Count: integer )"
	cases := []struct{ name, stmt, mf string }{
		{"CreateChangeCommit",
			"CREATE MICROFLOW MyFirstModule.MfCCC () BEGIN\n" +
				"$New = create MyFirstModule.Thing (Name = 'x', Count = 5);\n" +
				"change $New (Count = 6);\n" +
				"commit $New;\n" +
				"END", "MfCCC"},
		{"CommitWithEvents",
			"CREATE MICROFLOW MyFirstModule.MfCE (Item: MyFirstModule.Thing) BEGIN commit $Item with events; END", "MfCE"},
		{"Delete",
			"CREATE MICROFLOW MyFirstModule.MfDel (Item: MyFirstModule.Thing) BEGIN delete $Item; END", "MfDel"},
		{"Rollback",
			"CREATE MICROFLOW MyFirstModule.MfRb (Item: MyFirstModule.Thing) BEGIN rollback $Item; END", "MfRb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run := func(eng Engine) string {
				p := copyProject(t)
				if _, e := Run(Legacy, p, setup); e != nil {
					t.Fatalf("setup: %v", e)
				}
				if _, e := Run(eng, p, c.stmt); e != nil {
					t.Fatalf("%s create: %v", eng, e)
				}
				s, e := MicroflowCanonBSON(p, "MyFirstModule", c.mf)
				if e != nil {
					t.Fatalf("%s canon: %v", eng, e)
				}
				return s
			}
			if leg, msd := run(Legacy), run(ModelSDK); leg != msd {
				t.Errorf("%s divergence:\nlegacy:   %s\nmodelsdk: %s", c.name, leg, msd)
			}
		})
	}
}

// TestWriteParity_Microflow validates the codec-native microflow CREATE path
// against legacy, group by group. Skeleton = start → end, boolean return.
func TestWriteParity_Microflow(t *testing.T) {
	cases := []struct{ name, stmt, mf string }{
		{"Skeleton", "CREATE MICROFLOW MyFirstModule.MfEmpty () RETURNS BOOLEAN BEGIN RETURN true END", "MfEmpty"},
		{"Parameters", "CREATE MICROFLOW MyFirstModule.MfParams (Count: integer, Label: string) RETURNS BOOLEAN BEGIN RETURN true END", "MfParams"},
		{"VoidReturn", "CREATE MICROFLOW MyFirstModule.MfVoid () BEGIN END", "MfVoid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run := func(eng Engine) string {
				p := copyProject(t)
				if _, e := Run(eng, p, c.stmt); e != nil {
					t.Fatalf("%s create: %v", eng, e)
				}
				s, e := MicroflowCanonBSON(p, "MyFirstModule", c.mf)
				if e != nil {
					t.Fatalf("%s canon: %v", eng, e)
				}
				return s
			}
			if leg, msd := run(Legacy), run(ModelSDK); leg != msd {
				t.Errorf("%s divergence:\nlegacy:   %s\nmodelsdk: %s", c.name, leg, msd)
			}
		})
	}
}
