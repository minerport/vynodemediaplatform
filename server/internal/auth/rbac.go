package auth

type Role string
type Capability string

const (
	RoleOwner Role = "OWNER"
	RoleAdmin Role = "ADMIN"
	RoleUser  Role = "USER"
)
const (
	CapServerManage       Capability = "server.manage"
	CapUsersManage        Capability = "users.manage"
	CapSessionsManage     Capability = "sessions.manage"
	CapSecurityView       Capability = "security.view"
	CapAuditView          Capability = "audit.view"
	CapLibrariesView      Capability = "libraries.view"
	CapLibrariesManage    Capability = "libraries.manage"
	CapLibrariesScan      Capability = "libraries.scan"
	CapMediaInventoryView Capability = "media.inventory.view"
)

var grants = map[Role]map[Capability]bool{RoleOwner: {CapServerManage: true, CapUsersManage: true, CapSessionsManage: true, CapSecurityView: true, CapAuditView: true, CapLibrariesView: true, CapLibrariesManage: true, CapLibrariesScan: true, CapMediaInventoryView: true}, RoleAdmin: {CapUsersManage: true, CapSessionsManage: true, CapSecurityView: true, CapAuditView: true, CapLibrariesView: true, CapLibrariesManage: true, CapLibrariesScan: true, CapMediaInventoryView: true}, RoleUser: {}}

func Allowed(role Role, capability Capability) bool { return grants[role][capability] }
