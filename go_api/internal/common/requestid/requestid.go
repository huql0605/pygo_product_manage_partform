package requestid

import "context"

// ctxKey is an unexported type, so this key can never collide with a key
// defined in another package (which can happen with a plain string key).
type ctxKey struct{}

var key ctxKey

// WithContext returns a copy of ctx that carries the given request ID.
func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, key, id)
}

// FromContext returns the request ID stored in ctx, or "" when absent.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(key).(string); ok {
		return id
	}
	return ""
}
