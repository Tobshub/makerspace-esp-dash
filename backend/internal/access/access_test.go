package access

import "testing"

func TestRoles(t *testing.T) {
	if CanWrite(RoleViewer) {
		t.Fatal("viewer must be read-only")
	}
	if !CanWrite(RoleMember) || !CanWrite(RoleAdmin) || !CanWrite(RoleOwner) {
		t.Fatal("member, admin, and owner can write")
	}
	if CanManage(RoleMember) || CanManage(RoleViewer) {
		t.Fatal("only owner and admin manage membership")
	}
	if !ValidRole(RoleOwner) || ValidRole("superuser") {
		t.Fatal("role set")
	}
}
