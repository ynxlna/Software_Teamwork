package service

import (
	"context"
	"strings"
)

type requestIDContextKey struct{}
type userIDContextKey struct{}
type userRolesContextKey struct{}
type userPermissionsContextKey struct{}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey{}).(string)
	return value
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey{}, strings.TrimSpace(userID))
}

func UserIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(userIDContextKey{}).(string)
	return value
}

func WithUserRoles(ctx context.Context, roles string) context.Context {
	return context.WithValue(ctx, userRolesContextKey{}, strings.TrimSpace(roles))
}

func UserRolesFromContext(ctx context.Context) string {
	value, _ := ctx.Value(userRolesContextKey{}).(string)
	return value
}

func WithUserPermissions(ctx context.Context, permissions string) context.Context {
	return context.WithValue(ctx, userPermissionsContextKey{}, strings.TrimSpace(permissions))
}

func UserPermissionsFromContext(ctx context.Context) string {
	value, _ := ctx.Value(userPermissionsContextKey{}).(string)
	return value
}
