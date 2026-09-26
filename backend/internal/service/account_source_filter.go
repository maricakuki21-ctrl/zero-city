package service

import "context"

type accountSourceFilterKey struct{}

// WithAccountSourceFilter scopes administrative list/export queries only.
func WithAccountSourceFilter(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, accountSourceFilterKey{}, source)
}

func AccountSourceFilter(ctx context.Context) string {
	source, _ := ctx.Value(accountSourceFilterKey{}).(string)
	return source
}

func ValidAccountSourceFilter(source string) bool {
	return source == "" || source == "direct" || source == "shared"
}
