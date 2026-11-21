package auth

// Admin authorization (ADR-0015, audit BE-H6).
//
// Permissions use the names of the admin app (src/admin/src/stores/admin.ts) so the
// two sides cannot drift; TestRolePermissionsMatchFrontend compares the tables.
// The role is read from admin_users on every request, never from the token.

// Permission is one capability an admin role may hold.
type Permission string

const (
	PermKYCReview      Permission = "kyc.review"
	PermFraudResolve   Permission = "fraud.resolve"
	PermWorkflowAction Permission = "workflow.action"
	PermReconRun       Permission = "recon.run"
	PermOrgsEdit       Permission = "orgs.edit"
	PermAIManage       Permission = "ai.manage"
	PermAdminsManage   Permission = "admins.manage"
)

// RolePermissions is the single server-side role table.
var RolePermissions = map[string][]Permission{
	"viewer":       {},
	"kyc_reviewer": {PermKYCReview},
	"risk_officer": {PermFraudResolve, PermWorkflowAction},
	"ops":          {PermWorkflowAction, PermReconRun},
	"superadmin": {
		PermKYCReview, PermFraudResolve, PermWorkflowAction, PermReconRun,
		PermOrgsEdit, PermAIManage, PermAdminsManage,
	},
}

// RoleHas reports whether role holds permission p. An unknown role holds nothing.
func RoleHas(role string, p Permission) bool {
	for _, have := range RolePermissions[role] {
		if have == p {
			return true
		}
	}
	return false
}

// RoleHasAny reports whether role holds at least one of ps.
func RoleHasAny(role string, ps ...Permission) bool {
	for _, p := range ps {
		if RoleHas(role, p) {
			return true
		}
	}
	return false
}

// RoleHasAnyPermission is true for any role that holds at least one permission
// (a viewer holds none, so it sees nothing).
func RoleHasAnyPermission(role string) bool { return len(RolePermissions[role]) > 0 }
