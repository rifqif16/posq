package domain

import "testing"

func TestCanMatrix(t *testing.T) {
	cases := []struct {
		role Role
		perm string
		want bool
	}{
		{RoleOwner, "billing:manage", true},
		{RoleOwner, "report:profit_read", true},
		{RoleAdmin, "product:write", true},
		{RoleAdmin, "billing:manage", false},
		{RoleAdmin, "store:manage", false},
		{RoleAdmin, "report:profit_read", false},
		{RoleCashier, "product:read", true},
		{RoleCashier, "product:write", false},
		{RoleCashier, "product:cost_price_read", false},
		{RoleCashier, "sale:price_override", false},
		{RoleKitchen, "kds:update_status", true},
		{RoleKitchen, "product:read", false},
		{Role("hacker"), "product:read", false},
		{RoleAdmin, "", false},
		{RoleOwner, "", false},
	}
	for _, c := range cases {
		if got := Can(c.role, c.perm); got != c.want {
			t.Errorf("Can(%q, %q) = %v, want %v", c.role, c.perm, got, c.want)
		}
	}
}
