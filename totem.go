package totem

import "context"

type totemKey struct{}

func addTotemToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, totemKey{}, true)
}

func CheckContext(ctx context.Context) bool {
	_, ok := ctx.Value(totemKey{}).(bool)
	return ok
}
