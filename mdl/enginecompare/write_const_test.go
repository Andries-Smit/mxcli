package enginecompare

import "testing"

// TestWriteParity_Constant exercises constant writes (top-level documents) through
// the codec engine vs legacy: CREATE across types and CREATE OR MODIFY (which
// routes through UpdateConstant). Both engines must produce identical canonical BSON.
func TestWriteParity_Constant(t *testing.T) {
	cases := []struct{ name, create, modify, cname string }{
		{"String", "CREATE CONSTANT MyFirstModule.Endpoint TYPE string DEFAULT 'https://api.example.com'", "", "Endpoint"},
		{"Integer", "CREATE CONSTANT MyFirstModule.MaxRetries TYPE integer DEFAULT 5", "", "MaxRetries"},
		{"Decimal", "CREATE CONSTANT MyFirstModule.TaxRate TYPE decimal DEFAULT 0.21", "", "TaxRate"},
		{"Boolean", "CREATE CONSTANT MyFirstModule.Flag TYPE boolean DEFAULT true", "", "Flag"},
		{"Modify",
			"CREATE CONSTANT MyFirstModule.Endpoint2 TYPE string DEFAULT 'https://old'",
			"CREATE OR MODIFY CONSTANT MyFirstModule.Endpoint2 TYPE string DEFAULT 'https://new'",
			"Endpoint2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run := func(eng Engine) string {
				p := copyProject(t)
				if _, e := Run(eng, p, c.create); e != nil {
					t.Fatalf("%s create: %v", eng, e)
				}
				if c.modify != "" {
					if _, e := Run(eng, p, c.modify); e != nil {
						t.Fatalf("%s modify: %v", eng, e)
					}
				}
				s, e := ConstCanonBSON(p, "MyFirstModule", c.cname)
				if e != nil {
					t.Fatalf("%s canon: %v", eng, e)
				}
				return s
			}
			if leg, msd := run(Legacy), run(ModelSDK); leg != msd {
				t.Errorf("constant divergence:\nlegacy:   %s\nmodelsdk: %s", leg, msd)
			}
		})
	}
}

// TestWriteParity_DropConstant verifies DROP CONSTANT removes the unit in both engines.
func TestWriteParity_DropConstant(t *testing.T) {
	const create = "CREATE CONSTANT MyFirstModule.GoneC TYPE string DEFAULT 'x'"
	const drop = "DROP CONSTANT MyFirstModule.GoneC"
	for _, eng := range []Engine{Legacy, ModelSDK} {
		p := copyProject(t)
		if _, e := Run(eng, p, create); e != nil {
			t.Fatalf("%s create: %v", eng, e)
		}
		if _, e := Run(eng, p, drop); e != nil {
			t.Fatalf("%s drop: %v", eng, e)
		}
		if _, e := ConstCanonBSON(p, "MyFirstModule", "GoneC"); e == nil {
			t.Errorf("%s: constant still present after DROP", eng)
		}
	}
}
