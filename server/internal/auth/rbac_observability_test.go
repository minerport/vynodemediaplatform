package auth

import "testing"

func TestObservabilityCapabilitiesAreAdministrative(t *testing.T) {
	for _, role := range []Role{RoleOwner, RoleAdmin} {
		if !Allowed(role, CapObservabilityView) || !Allowed(role, CapObservabilityManage) {
			t.Fatalf("%s lacks observability", role)
		}
	}
	if Allowed(RoleUser, CapObservabilityView) || Allowed(RoleUser, CapObservabilityManage) {
		t.Fatal("ordinary user received server observability")
	}
}
