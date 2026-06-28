package service

import "strings"

type RequestContext struct {
	RequestID      string
	UserID         string
	Roles          []string
	Permissions    []string
	ForwardedFor   string
	ForwardedProto string
}

func validateActor(reqCtx RequestContext) error {
	if strings.TrimSpace(reqCtx.UserID) == "" {
		return UnauthorizedError()
	}
	return nil
}

func hasPermission(reqCtx RequestContext, permission string) bool {
	for _, candidate := range reqCtx.Permissions {
		if strings.TrimSpace(candidate) == permission {
			return true
		}
	}
	return false
}
