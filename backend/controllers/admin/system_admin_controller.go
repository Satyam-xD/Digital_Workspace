package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"backend/config"
	"backend/middleware"
	"backend/models"
	"backend/socket"
	"backend/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetPlatformStats computes platform overview metrics
// GET /api/admin/system/stats
func GetPlatformStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userColl := config.DB.Collection("users")
	teamColl := config.DB.Collection("teams")
	taskColl := config.DB.Collection("tasks")
	docColl := config.DB.Collection("documents")

	totalUsers, _ := userColl.CountDocuments(ctx, bson.M{"status": bson.M{"$ne": models.StatusPending}})
	activeUsers, _ := userColl.CountDocuments(ctx, bson.M{"status": models.StatusActive})
	suspendedUsers, _ := userColl.CountDocuments(ctx, bson.M{"status": models.StatusSuspended})
	pendingUsers, _ := userColl.CountDocuments(ctx, bson.M{"status": models.StatusPending})
	totalTeams, _ := teamColl.CountDocuments(ctx, bson.M{})
	totalTasks, _ := taskColl.CountDocuments(ctx, bson.M{})
	completedTasks, _ := taskColl.CountDocuments(ctx, bson.M{"status": "Done"})
	totalDocuments, _ := docColl.CountDocuments(ctx, bson.M{})
	teamHeads, _ := userColl.CountDocuments(ctx, bson.M{"role": models.RoleTeamHead})
	teamMembers, _ := userColl.CountDocuments(ctx, bson.M{"role": models.RoleTeamMember})

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"totalUsers":     totalUsers,
		"activeUsers":    activeUsers,
		"suspendedUsers": suspendedUsers,
		"pendingUsers":   pendingUsers,
		"totalTeams":     totalTeams,
		"totalTasks":     totalTasks,
		"completedTasks": completedTasks,
		"totalDocuments": totalDocuments,
		"teamHeads":      teamHeads,
		"teamMembers":    teamMembers,
	})
}

// GetAuditLogs fetches recent system audit logs
// GET /api/admin/system/audit-logs
func GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auditColl := config.DB.Collection("auditlogs")
	userColl := config.DB.Collection("users")
	teamColl := config.DB.Collection("teams")

	opts := options.Find().SetSort(bson.M{"createdAt": -1}).SetLimit(100)
	cursor, err := auditColl.Find(ctx, bson.M{}, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load audit logs")
		return
	}
	defer cursor.Close(ctx)

	var logs []models.AuditLog
	_ = cursor.All(ctx, &logs)

	for i := range logs {
		var adm models.UserSummary
		if err := userColl.FindOne(ctx, bson.M{"_id": logs[i].Admin}).Decode(&adm); err == nil {
			logs[i].AdminDetails = &adm
		}
		if logs[i].TargetUser != nil {
			var tu models.UserSummary
			if err := userColl.FindOne(ctx, bson.M{"_id": logs[i].TargetUser}).Decode(&tu); err == nil {
				logs[i].TargetUserDetails = &tu
			}
		}
		if logs[i].TargetTeam != nil {
			var tt struct {
				Name string `json:"name"`
			}
			if err := teamColl.FindOne(ctx, bson.M{"_id": logs[i].TargetTeam}).Decode(&tt); err == nil {
				logs[i].TargetTeamDetails = tt
			}
		}
	}

	utils.WriteJSON(w, http.StatusOK, logs)
}

// SendPlatformBroadcast broadcasts an urgent announcement to all clients
// POST /api/admin/system/broadcast
func SendPlatformBroadcast(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req struct {
		Title   string `json:"title"`
		Message string `json:"message"`
		Type    string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" || req.Message == "" {
		utils.WriteError(w, http.StatusBadRequest, "Title and message are required")
		return
	}

	broadcastType := "info"
	if req.Type != "" {
		broadcastType = req.Type
	}

	socket.BroadcastGlobal("system_broadcast", map[string]any{
		"title":     req.Title,
		"message":   req.Message,
		"type":      broadcastType,
		"sender":    user.Name,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})

	auditColl := config.DB.Collection("auditlogs")
	now := time.Now()
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:        primitive.NewObjectID(),
		Admin:     user.ID,
		Action:    "PLATFORM_BROADCAST_SENT",
		Details:   fmt.Sprintf("Broadcast: %s", req.Title),
		CreatedAt: now,
		UpdatedAt: now,
	})

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "Broadcast sent"})
}
