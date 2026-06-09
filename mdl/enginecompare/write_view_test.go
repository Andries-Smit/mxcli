package enginecompare

import "testing"

// TestWriteParity_ViewEntity verifies a view entity (OQL source document + the
// entity's OqlViewEntitySource + OqlViewValue attributes) matches legacy.
func TestWriteParity_ViewEntity(t *testing.T) {
	const script = "CREATE MODULE VM; " +
		"CREATE PERSISTENT ENTITY VM.Product ( Name: string(100), Price: decimal, IsActive: boolean default true ); " +
		"CREATE VIEW ENTITY VM.ActiveProducts ( Name: string(100), Price: decimal ) AS " +
		"select p.Name as Name, p.Price as Price from VM.Product as p where p.IsActive = true"
	run := func(eng Engine) string {
		p := copyProject(t)
		if _, e := Run(eng, p, script); e != nil {
			t.Fatalf("%s: %v", eng, e)
		}
		s, e := EntityCanonBSON(p, "VM", "ActiveProducts")
		if e != nil {
			t.Fatalf("%s canon: %v", eng, e)
		}
		return s
	}
	if leg, msd := run(Legacy), run(ModelSDK); leg != msd {
		t.Errorf("view-entity divergence:\nlegacy:   %s\nmodelsdk: %s", leg, msd)
	}
}
