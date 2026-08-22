package auth

type Role string
type Capability string

const (
	RoleOwner Role = "OWNER"
	RoleAdmin Role = "ADMIN"
	RoleUser  Role = "USER"
)
const (
	CapServerManage   Capability = "server.manage"
	CapUsersManage    Capability = "users.manage"
	CapSessionsManage Capability = "sessions.manage"
	CapSecurityView   Capability = "security.view"
	CapAuditView      Capability = "audit.view"
)

var grants = map[Role]map[Capability]bool{RoleOwner: {CapServerManage: true, CapUsersManage: true, CapSessionsManage: true, CapSecurityView: true, CapAuditView: true}, RoleAdmin: {CapUsersManage: true, CapSessionsManage: true, CapSecurityView: true, CapAuditView: true}, RoleUser: {}}

func Allowed(role Role, capability Capability) bool { return grants[role][capability] }
