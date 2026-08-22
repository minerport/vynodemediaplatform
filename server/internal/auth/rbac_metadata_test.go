package auth

import "testing"

func TestMetadataCapabilities(t *testing.T) {
	for _, role := range []Role{RoleOwner, RoleAdmin} {
		if !Allowed(role, CapMetadataManage) || !Allowed(role, CapProviderManage) || !Allowed(role, CapLogicalMediaView) {
			t.Fatalf("%s missing metadata capabilities", role)
		}
	}
	if !Allowed(RoleUser, CapLogicalMediaView) {
		t.Fatal("user cannot browse logical media")
	}
	if Allowed(RoleUser, CapMetadataManage) || Allowed(RoleUser, CapProviderManage) || Allowed(RoleUser, CapMediaInventoryView) {
		t.Fatal("user received metadata administration or physical inventory")
	}
}
