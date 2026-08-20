package security

var rolePermissions = map[string]map[string]bool{"OWNER": {"*": true}, "ADMIN": {"*": true}, "RECOVERY_ENGINEER": {"backup_source.read": true, "backup_source.manage": true, "recovery_policy.read": true, "recovery_policy.manage": true, "drill.read": true, "drill.run": true, "evidence.read": true, "report.export": true}, "OPERATOR": {"backup_source.read": true, "recovery_policy.read": true, "drill.read": true, "drill.run": true, "evidence.read": true}, "AUDITOR": {"backup_source.read": true, "recovery_policy.read": true, "drill.read": true, "evidence.read": true, "report.export": true, "audit.read": true}, "VIEWER": {"backup_source.read": true, "recovery_policy.read": true, "drill.read": true, "evidence.read": true}}

func Allowed(roles []string, permission string) bool {
	for _, role := range roles {
		if rolePermissions[role]["*"] || rolePermissions[role][permission] {
			return true
		}
	}
	return false
}
