package auth

import "testing"

func TestCurationCapabilities(t *testing.T) {
	for _, r := range []Role{RoleOwner, RoleAdmin} {
		if !Allowed(r, CapCollectionsManage) || !Allowed(r, CapCurationSelfManage) {
			t.Fatalf("%s missing curation grants", r)
		}
	}
	if Allowed(RoleUser, CapCollectionsManage) || !Allowed(RoleUser, CapCurationSelfManage) {
		t.Fatal("ordinary user curation grants invalid")
	}
}
