package services

import (
	"testing"

	storageModels "github.com/juggleim/jugglemate-server/storages/models"
)

func TestRoleToStrings(t *testing.T) {
	if got := RoleToStrings(storageModels.UserRoleAdmin); len(got) != 1 || got[0] != "admin" {
		t.Fatalf("admin role = %v, want [admin]", got)
	}
	if got := RoleToStrings(storageModels.UserRoleCustomerService); len(got) != 1 || got[0] != "customer" {
		t.Fatalf("customer service role = %v, want [customer]", got)
	}
}

func TestRolesToStorageRole(t *testing.T) {
	if got := RolesToStorageRole([]string{"customer", "admin"}); got != storageModels.UserRoleAdmin {
		t.Fatalf("roles with admin = %d, want admin", got)
	}
	if got := RolesToStorageRole([]string{"customer"}); got != storageModels.UserRoleCustomerService {
		t.Fatalf("customer only = %d, want customer service", got)
	}
}

func TestFilterRoleToStorageRole(t *testing.T) {
	admin := FilterRoleToStorageRole("admin")
	if admin == nil || *admin != storageModels.UserRoleAdmin {
		t.Fatalf("filter admin = %v", admin)
	}
	customer := FilterRoleToStorageRole("customer")
	if customer == nil || *customer != storageModels.UserRoleCustomerService {
		t.Fatalf("filter customer = %v", customer)
	}
	if FilterRoleToStorageRole("") != nil {
		t.Fatalf("empty filter should be nil")
	}
}

func TestToUserItem(t *testing.T) {
	item := ToUserItem(&storageModels.User{
		UserId:       "u_1",
		LoginAccount: "alice01",
		Avator:       "avatar.png",
		Email:        "alice@example.com",
		Role:         storageModels.UserRoleAdmin,
	})
	if item.ID != "u_1" || item.Username != "alice01" || item.Email != "alice@example.com" {
		t.Fatalf("item = %+v", item)
	}
	if len(item.Roles) != 1 || item.Roles[0] != "admin" {
		t.Fatalf("roles = %v", item.Roles)
	}
	if item.Permissions == nil {
		t.Fatalf("permissions should be non-nil slice")
	}
}
