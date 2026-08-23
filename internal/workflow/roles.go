package workflow

import (
	"strings"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
)

type Role string

const (
	RoleSafetyOfficer Role = "safety_officer"
	RoleReviewer      Role = "reviewer"
)

func ParseRole(value string) (Role, error) {
	role := Role(strings.TrimSpace(value))
	if role != RoleSafetyOfficer && role != RoleReviewer {
		return "", domain.NewRuleError("invalid_role", "角色必须是 safety_officer 或 reviewer", "role")
	}
	return role, nil
}

func requireRole(actual Role, expected Role) error {
	if actual != expected {
		return domain.NewRuleError("forbidden", "当前角色无权执行该操作", "role")
	}
	return nil
}
