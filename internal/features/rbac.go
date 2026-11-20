package features

import (
	"crypto/subtle"
	"net/http"
)

// Role defines control-plane RBAC permission scopes.
type Role string

const (
	RoleNone    Role = ""
	RoleAuditor Role = "auditor"
	RoleAdmin   Role = "admin"
)

// RBACAuthorizer resolves full admin vs. read-only auditor control-plane tokens.
type RBACAuthorizer struct {
	AdminToken   string
	AuditorToken string
}

// ResolveRole returns the RBAC role for a presented bearer token.
func (a RBACAuthorizer) ResolveRole(token string) Role {
	if token == "" {
		return RoleNone
	}
	if a.AdminToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(a.AdminToken)) == 1 {
		return RoleAdmin
	}
	if a.AuditorToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(a.AuditorToken)) == 1 {
		return RoleAuditor
	}
	return RoleNone
}

// Allowed reports whether role may execute the given HTTP method on control-plane routes.
func (a RBACAuthorizer) Allowed(role Role, method string) bool {
	switch role {
	case RoleAdmin:
		return true
	case RoleAuditor:
		return method == http.MethodGet || method == http.MethodHead
	default:
		return false
	}
}
