package utils

import "context"

type requestIDKey struct{}

// WithRequestID returns a context carrying the request id so code that only has a
// context (for example database callbacks) can attribute work to a request.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// RequestIDFromContext returns the request id stored by WithRequestID, or "".
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
