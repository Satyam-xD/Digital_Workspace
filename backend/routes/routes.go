package routes

import (
	"net/http"

	"backend/controllers"
	"backend/controllers/admin"
	"backend/middleware"
)

// RegisterRoutes registers all REST API routes onto the given ServeMux
func RegisterRoutes(mux *http.ServeMux) {
	// ==================== AUTH ROUTES ====================
	mux.HandleFunc("POST /api/auth/register", middleware.StrictRateLimit.Middleware(controllers.RegisterUser))
	mux.HandleFunc("POST /api/auth/login", middleware.StrictRateLimit.Middleware(controllers.AuthUser))
	mux.HandleFunc("GET /api/auth/profile", middleware.Protect(controllers.GetUserProfile))
	mux.HandleFunc("GET /api/auth/users", middleware.Protect(controllers.GetAllUsers))

	// ==================== TASK ROUTES ====================
	mux.HandleFunc("GET /api/tasks", middleware.Protect(controllers.GetTasks))
	mux.HandleFunc("POST /api/tasks", middleware.Protect(controllers.SetTask))
	mux.HandleFunc("PUT /api/tasks/{id}", middleware.Protect(controllers.UpdateTask))
	mux.HandleFunc("DELETE /api/tasks/{id}", middleware.Protect(controllers.DeleteTask))

	// ==================== TEAM ROUTES ====================
	mux.HandleFunc("GET /api/team", middleware.Protect(controllers.GetTeamMembers))
	mux.HandleFunc("POST /api/team", middleware.Protect(controllers.AddTeamMember))
	mux.HandleFunc("POST /api/team/create", middleware.Admin(controllers.CreateNewTeam))
	mux.HandleFunc("POST /api/team/members", middleware.Protect(controllers.AddTeamMember))
	mux.HandleFunc("PUT /api/team", middleware.Admin(controllers.UpdateTeamDetails))
	mux.HandleFunc("PUT /api/team/details", middleware.Admin(controllers.UpdateTeamDetails))
	mux.HandleFunc("DELETE /api/team/delete/{teamId}", middleware.Admin(controllers.DeleteTeam))
	mux.HandleFunc("DELETE /api/team/{teamId}/member/{memberId}", middleware.Admin(controllers.RemoveTeamMember))
	mux.HandleFunc("DELETE /api/team/members/{id}", middleware.Admin(controllers.RemoveTeamMember))
	mux.HandleFunc("GET /api/team/activity/{teamId}", middleware.Protect(controllers.GetTeamActivity))
	mux.HandleFunc("GET /api/team/activities", middleware.Protect(controllers.GetTeamActivities))

	// ==================== DOCUMENT ROUTES ====================
	mux.HandleFunc("GET /api/documents", middleware.Protect(controllers.GetDocuments))
	mux.HandleFunc("POST /api/documents/upload", middleware.Protect(controllers.UploadDocument))
	mux.HandleFunc("GET /api/documents/download/{id}", controllers.DownloadDocument)
	mux.HandleFunc("GET /api/documents/view/{id}", controllers.ViewDocument)
	mux.HandleFunc("POST /api/documents/folder", middleware.Protect(controllers.CreateFolder))
	mux.HandleFunc("PUT /api/documents/folder/{id}", middleware.Protect(controllers.RenameFolder))
	mux.HandleFunc("DELETE /api/documents/folder/{id}", middleware.Protect(controllers.DeleteFolder))
	mux.HandleFunc("DELETE /api/documents/{id}", middleware.Protect(controllers.DeleteDocument))
	mux.HandleFunc("PATCH /api/documents/{id}/downloadable", middleware.Admin(controllers.ToggleDownloadable))

	// ==================== PASSWORD ROUTES ====================
	mux.HandleFunc("GET /api/passwords", middleware.Protect(controllers.GetPasswords))
	mux.HandleFunc("POST /api/passwords", middleware.Protect(controllers.CreatePassword))
	mux.HandleFunc("PUT /api/passwords/{id}", middleware.Protect(controllers.UpdatePassword))
	mux.HandleFunc("DELETE /api/passwords/{id}", middleware.Protect(controllers.DeletePassword))

	// ==================== CHAT ROUTES ====================
	mux.HandleFunc("GET /api/chat", middleware.Protect(controllers.FetchChats))
	mux.HandleFunc("POST /api/chat", middleware.Protect(controllers.AccessChat))
	mux.HandleFunc("POST /api/chat/group", middleware.Protect(controllers.CreateGroupChat))
	mux.HandleFunc("PUT /api/chat/rename", middleware.Protect(controllers.RenameGroup))
	mux.HandleFunc("PUT /api/chat/groupadd", middleware.Protect(controllers.AddToGroup))
	mux.HandleFunc("PUT /api/chat/groupremove", middleware.Protect(controllers.RemoveFromGroup))
	mux.HandleFunc("POST /api/chat/message", middleware.Protect(controllers.SendMessage))
	mux.HandleFunc("POST /api/chat/upload", middleware.Protect(controllers.UploadChatAttachment))
	mux.HandleFunc("GET /api/chat/{room}", middleware.Protect(controllers.GetChatHistory))
	mux.HandleFunc("GET /api/chat/history/{room}", middleware.Protect(controllers.GetChatHistory))

	// ==================== EVENT ROUTES ====================
	mux.HandleFunc("GET /api/events", middleware.Protect(controllers.GetEvents))
	mux.HandleFunc("POST /api/events", middleware.Protect(controllers.CreateEvent))
	mux.HandleFunc("PUT /api/events/{id}", middleware.Protect(controllers.UpdateEvent))
	mux.HandleFunc("DELETE /api/events/{id}", middleware.Protect(controllers.DeleteEvent))

	// ==================== NOTIFICATION ROUTES ====================
	mux.HandleFunc("GET /api/notifications", middleware.Protect(controllers.GetNotifications))
	mux.HandleFunc("GET /api/notifications/count", middleware.Protect(controllers.GetUnreadCount))
	mux.HandleFunc("PUT /api/notifications/read-all", middleware.Protect(controllers.MarkAllAsRead))
	mux.HandleFunc("DELETE /api/notifications/clear-all", middleware.Protect(controllers.ClearAllNotifications))
	mux.HandleFunc("DELETE /api/notifications/clear-read", middleware.Protect(controllers.ClearReadNotifications))
	mux.HandleFunc("PUT /api/notifications/{id}/read", middleware.Protect(controllers.MarkAsRead))
	mux.HandleFunc("DELETE /api/notifications/{id}", middleware.Protect(controllers.DeleteNotification))

	// ==================== MASTER ADMIN ROUTES ====================
	// System Admin
	mux.HandleFunc("GET /api/admin/stats", middleware.MasterAdmin(admin.GetPlatformStats))
	mux.HandleFunc("GET /api/admin/system/stats", middleware.MasterAdmin(admin.GetPlatformStats))
	mux.HandleFunc("GET /api/admin/audit-logs", middleware.MasterAdmin(admin.GetAuditLogs))
	mux.HandleFunc("GET /api/admin/system/audit-logs", middleware.MasterAdmin(admin.GetAuditLogs))
	mux.HandleFunc("POST /api/admin/broadcast", middleware.MasterAdmin(admin.SendPlatformBroadcast))
	mux.HandleFunc("POST /api/admin/system/broadcast", middleware.MasterAdmin(admin.SendPlatformBroadcast))

	// Team Admin
	mux.HandleFunc("GET /api/admin/teams", middleware.MasterAdmin(admin.GetAllTeamsAdmin))
	mux.HandleFunc("POST /api/admin/teams", middleware.MasterAdmin(admin.CreateTeamAdmin))
	mux.HandleFunc("PUT /api/admin/teams/{teamId}", middleware.MasterAdmin(admin.UpdateTeamDetailsAdmin))
	mux.HandleFunc("PUT /api/admin/teams/{teamId}/details", middleware.MasterAdmin(admin.UpdateTeamDetailsAdmin))
	mux.HandleFunc("DELETE /api/admin/teams/{teamId}", middleware.MasterAdmin(admin.DeleteTeamAdmin))
	mux.HandleFunc("PATCH /api/admin/teams/{teamId}/transfer", middleware.MasterAdmin(admin.TransferTeamOwnership))
	mux.HandleFunc("PUT /api/admin/teams/{teamId}/transfer-ownership", middleware.MasterAdmin(admin.TransferTeamOwnership))
	mux.HandleFunc("POST /api/admin/teams/{teamId}/members", middleware.MasterAdmin(admin.AddMemberToTeamAdmin))
	mux.HandleFunc("DELETE /api/admin/teams/{teamId}/members/{memberId}", middleware.MasterAdmin(admin.RemoveMemberFromTeamAdmin))

	// User Admin
	mux.HandleFunc("GET /api/admin/users", middleware.MasterAdmin(admin.GetAllUsersAdmin))
	mux.HandleFunc("GET /api/admin/users/pending", middleware.MasterAdmin(admin.GetPendingUsers))
	mux.HandleFunc("PATCH /api/admin/users/bulk-status", middleware.MasterAdmin(admin.BulkUpdateUserStatus))
	mux.HandleFunc("PUT /api/admin/users/{userId}", middleware.MasterAdmin(admin.UpdateUserAdmin))
	mux.HandleFunc("DELETE /api/admin/users/{userId}", middleware.MasterAdmin(admin.DeleteUserAdmin))
	mux.HandleFunc("PATCH /api/admin/users/{userId}/role", middleware.MasterAdmin(admin.UpdateUserRole))
	mux.HandleFunc("PUT /api/admin/users/{userId}/role", middleware.MasterAdmin(admin.UpdateUserRole))
	mux.HandleFunc("PATCH /api/admin/users/{userId}/suspend", middleware.MasterAdmin(admin.ToggleUserSuspension))
	mux.HandleFunc("PUT /api/admin/users/{userId}/toggle-suspension", middleware.MasterAdmin(admin.ToggleUserSuspension))
	mux.HandleFunc("POST /api/admin/users/{userId}/approve", middleware.MasterAdmin(admin.ApproveUser))
	mux.HandleFunc("PUT /api/admin/users/{userId}/approve", middleware.MasterAdmin(admin.ApproveUser))
	mux.HandleFunc("DELETE /api/admin/users/{userId}/reject", middleware.MasterAdmin(admin.RejectUser))
	mux.HandleFunc("POST /api/admin/impersonate/{userId}", middleware.MasterAdmin(admin.ImpersonateUser))
	mux.HandleFunc("POST /api/admin/users/{userId}/impersonate", middleware.MasterAdmin(admin.ImpersonateUser))
	mux.HandleFunc("GET /api/admin/users/{userId}/timeline", middleware.MasterAdmin(admin.GetUserTimeline))
}
