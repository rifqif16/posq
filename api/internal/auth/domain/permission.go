package domain

var (
	adminPerms = toSet(
		"sale:create", "sale:hold_resume", "sale:discount", "sale:price_override", "sale:void",
		"sale:refund", "sale:read_all", "product:read", "product:write", "product:cost_price_read",
		"inventory:adjust", "inventory:opname", "supplier:manage", "purchase:manage", "expense:write",
		"cash:open_close_shift", "cash:in_out", "customer:read", "customer:write", "report:read",
		"user:manage", "settings:write", "order:create", "order:send_kitchen", "order:void_item",
		"table:manage", "variant:manage", "modifier:manage", "recipe:manage", "kds:update_status",
	)
	cashierPerms = toSet(
		"sale:create", "sale:hold_resume", "sale:discount", "sale:void", "sale:refund",
		"product:read", "cash:open_close_shift", "cash:in_out", "customer:read", "customer:write",
		"order:create", "order:send_kitchen", "order:void_item", "kds:update_status",
	)
	kitchenPerms = toSet("kds:update_status")
)

func toSet(items ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, i := range items {
		m[i] = struct{}{}
	}
	return m
}

func Can(role Role, permission string) bool {
	var set map[string]struct{}
	switch role {
	case RoleOwner:
		return permission != ""
	case RoleAdmin:
		set = adminPerms
	case RoleCashier:
		set = cashierPerms
	case RoleKitchen:
		set = kitchenPerms
	default:
		return false
	}
	_, ok := set[permission]
	return ok
}
