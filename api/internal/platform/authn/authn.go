package authn

import (
	"context"

	"github.com/google/uuid"
)

type Principal struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	Role     string
	Can      func(permission string) bool
}

func (p Principal) Has(permission string) bool {
	return p.Can != nil && p.Can(permission)
}

type ctxKey struct{}

func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
