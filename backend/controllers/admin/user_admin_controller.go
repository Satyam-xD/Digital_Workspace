package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"backend/config"
	"backend/middleware"
	"backend/models"
	"backend/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetAllUsersAdmin returns users with team information
// GET /api/admin/users
func GetAllUsersAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	search := r.URL.Query().Get("search")
	filter := bson.M{"status": bson.M{"$ne": models.StatusPending}}

	if search != "" {
		escaped := regexp.QuoteMeta(search)
		filter["$or"] = bson.A{
			bson.M{"name": bson.M{"$regex": escaped, "$options": "i"}},
			bson.M{"email": bson.M{"$regex": escaped, "$options": "i"}},
		}
	}

	userColl := config.DB.Collection("users")
	teamColl := config.DB.Collection("teams")

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := userColl.Find(ctx, filter, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load users")
		return
	}
	defer cursor.Close(ctx)

	var users []models.User
	_ = cursor.All(ctx, &users)

	for i := range users {
		users[i].Password = ""
		teamCursor, err := teamColl.Find(ctx, bson.M{
			"$or": bson.A{
				bson.M{"owner": users[i].ID},
				bson.M{"members": users[i].ID},
			},
		})
		if err == nil {
			var teams []models.Team
			_ = teamCursor.All(ctx, &teams)
			for _, t := range teams {
				users[i].Teams = append(users[i].Teams, t.Name)
			}
		}
	}

	utils.WriteJSON(w, http.StatusOK, users)
}

// GetPendingUsers returns users awaiting master admin approval
// GET /api/admin/users/pending
func GetPendingUsers(w http.ResponseWriter, r *http.Request) {
	userColl := config.DB.Collection("users")
	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := userColl.Find(r.Context(), bson.M{"status": models.StatusPending}, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load pending users")
		return
	}
	defer cursor.Close(r.Context())

	var users []models.User
	_ = cursor.All(r.Context(), &users)
	for i := range users {
		users[i].Password = ""
	}

	utils.WriteJSON(w, http.StatusOK, users)
}

// ApproveUser activates a pending user
// PUT /api/admin/users/:userId/approve
func ApproveUser(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	uIDStr := r.PathValue("userId")
	uObjID, err := primitive.ObjectIDFromHex(uIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	userColl := config.DB.Collection("users")
	var target models.User
	if err := userColl.FindOne(r.Context(), bson.M{"_id": uObjID, "status": models.StatusPending}).Decode(&target); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "User not found or not pending")
		return
	}

	_, _ = userColl.UpdateOne(r.Context(), bson.M{"_id": uObjID}, bson.M{"$set": bson.M{"status": models.StatusActive}})

	auditColl := config.DB.Collection("auditlogs")
	now := time.Now()
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:         primitive.NewObjectID(),
		Admin:      adminUser.ID,
		Action:     "USER_APPROVED",
		TargetUser: &uObjID,
		Details:    fmt.Sprintf("Approved %s", target.Email),
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "User approved"})
}

// RejectUser removes a pending user
// DELETE /api/admin/users/:userId/reject
func RejectUser(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	uIDStr := r.PathValue("userId")
	uObjID, err := primitive.ObjectIDFromHex(uIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	userColl := config.DB.Collection("users")
	var target models.User
	if err := userColl.FindOne(r.Context(), bson.M{"_id": uObjID, "status": models.StatusPending}).Decode(&target); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "User not found or not pending")
		return
	}

	_, _ = userColl.DeleteOne(r.Context(), bson.M{"_id": uObjID})

	auditColl := config.DB.Collection("auditlogs")
	now := time.Now()
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:        primitive.NewObjectID(),
		Admin:     adminUser.ID,
		Action:    "USER_REJECTED",
		Details:   fmt.Sprintf("Rejected registration for %s", target.Email),
		CreatedAt: now,
		UpdatedAt: now,
	})

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "User rejected"})
}

// UpdateUserRole modifies a user's role
// PUT /api/admin/users/:userId/role
func UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	uIDStr := r.PathValue("userId")
	uObjID, err := primitive.ObjectIDFromHex(uIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	validRoles := map[string]bool{
		"team_member":  true,
		"team_head":    true,
		"admin":        true,
		"master_admin": true,
	}
	if !validRoles[req.Role] {
		utils.WriteError(w, http.StatusBadRequest, "Invalid role")
		return
	}

	userColl := config.DB.Collection("users")
	var target models.User
	if err := userColl.FindOne(r.Context(), bson.M{"_id": uObjID}).Decode(&target); err != nil {
		utils.WriteError(w, http.StatusNotFound, "User not found")
		return
	}

	if target.ID == adminUser.ID && req.Role != "master_admin" {
		utils.WriteError(w, http.StatusBadRequest, "Cannot downgrade your own role")
		return
	}

	_, _ = userColl.UpdateOne(r.Context(), bson.M{"_id": uObjID}, bson.M{"$set": bson.M{"role": req.Role}})

	auditColl := config.DB.Collection("auditlogs")
	now := time.Now()
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:         primitive.NewObjectID(),
		Admin:      adminUser.ID,
		Action:     "ROLE_CHANGED",
		TargetUser: &uObjID,
		Details:    fmt.Sprintf("Role changed from %s to %s", target.Role, req.Role),
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	utils.EmitTeamUpdate("", "ROLE_UPDATE", []string{uIDStr})

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"_id":   target.ID,
		"name":  target.Name,
		"email": target.Email,
		"role":  req.Role,
	})
}

// ToggleUserSuspension toggles active / suspended status
// PUT /api/admin/users/:userId/toggle-suspension
func ToggleUserSuspension(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	uIDStr := r.PathValue("userId")
	uObjID, err := primitive.ObjectIDFromHex(uIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	userColl := config.DB.Collection("users")
	var target models.User
	if err := userColl.FindOne(r.Context(), bson.M{"_id": uObjID}).Decode(&target); err != nil {
		utils.WriteError(w, http.StatusNotFound, "User not found")
		return
	}

	newStatus := models.StatusSuspended
	action := "USER_SUSPENDED"
	if target.Status == models.StatusSuspended {
		newStatus = models.StatusActive
		action = "USER_ACTIVATED"
	}

	_, _ = userColl.UpdateOne(r.Context(), bson.M{"_id": uObjID}, bson.M{"$set": bson.M{"status": newStatus}})

	auditColl := config.DB.Collection("auditlogs")
	now := time.Now()
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:         primitive.NewObjectID(),
		Admin:      adminUser.ID,
		Action:     action,
		TargetUser: &uObjID,
		Details:    fmt.Sprintf("User status changed to %s", newStatus),
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"_id":    target.ID,
		"status": newStatus,
	})
}

// ImpersonateUser logs in as another user for troubleshooting
// POST /api/admin/users/:userId/impersonate
func ImpersonateUser(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	uIDStr := r.PathValue("userId")
	uObjID, err := primitive.ObjectIDFromHex(uIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if uObjID == adminUser.ID {
		utils.WriteError(w, http.StatusBadRequest, "Cannot impersonate yourself")
		return
	}

	userColl := config.DB.Collection("users")
	var target models.User
	if err := userColl.FindOne(r.Context(), bson.M{"_id": uObjID}).Decode(&target); err != nil {
		utils.WriteError(w, http.StatusNotFound, "User not found")
		return
	}

	token, err := utils.GenerateToken(target.ID.Hex())
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	auditColl := config.DB.Collection("auditlogs")
	now := time.Now()
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:         primitive.NewObjectID(),
		Admin:      adminUser.ID,
		Action:     "IMPERSONATION_STARTED",
		TargetUser: &uObjID,
		Details:    fmt.Sprintf("Started impersonating %s", target.Email),
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"_id":            target.ID,
		"name":           target.Name,
		"email":          target.Email,
		"role":           target.Role,
		"token":          token,
		"isImpersonated": true,
	})
}

// GetUserTimeline returns user audit log history
// GET /api/admin/users/:userId/timeline
func GetUserTimeline(w http.ResponseWriter, r *http.Request) {
	uIDStr := r.PathValue("userId")
	uObjID, err := primitive.ObjectIDFromHex(uIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	auditColl := config.DB.Collection("auditlogs")
	userColl := config.DB.Collection("users")

	filter := bson.M{
		"$or": bson.A{
			bson.M{"targetUser": uObjID},
			bson.M{"admin": uObjID},
		},
	}
	opts := options.Find().SetSort(bson.M{"createdAt": -1}).SetLimit(50)
	cursor, err := auditColl.Find(r.Context(), filter, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load timeline")
		return
	}
	defer cursor.Close(r.Context())

	var logs []models.AuditLog
	_ = cursor.All(r.Context(), &logs)

	for i := range logs {
		var adm models.UserSummary
		if err := userColl.FindOne(r.Context(), bson.M{"_id": logs[i].Admin}).Decode(&adm); err == nil {
			logs[i].AdminDetails = &adm
		}
		if logs[i].TargetUser != nil {
			var tu models.UserSummary
			if err := userColl.FindOne(r.Context(), bson.M{"_id": logs[i].TargetUser}).Decode(&tu); err == nil {
				logs[i].TargetUserDetails = &tu
			}
		}
	}

	utils.WriteJSON(w, http.StatusOK, logs)
}

// UpdateUserAdmin updates user name, email, or password
// PUT /api/admin/users/:userId
func UpdateUserAdmin(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	uIDStr := r.PathValue("userId")
	uObjID, err := primitive.ObjectIDFromHex(uIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	userColl := config.DB.Collection("users")
	var user models.User
	if err := userColl.FindOne(r.Context(), bson.M{"_id": uObjID}).Decode(&user); err != nil {
		utils.WriteError(w, http.StatusNotFound, "User not found")
		return
	}

	updateFields := bson.M{"updatedAt": time.Now()}
	if req.Email != "" && req.Email != user.Email {
		count, _ := userColl.CountDocuments(r.Context(), bson.M{"email": req.Email})
		if count > 0 {
			utils.WriteError(w, http.StatusBadRequest, "Email already in use")
			return
		}
		updateFields["email"] = req.Email
		user.Email = req.Email
	}

	if req.Name != "" {
		updateFields["name"] = req.Name
		user.Name = req.Name
	}

	if req.Password != "" {
		hashed, err := models.HashPassword(req.Password)
		if err == nil {
			updateFields["password"] = hashed
		}
	}

	_, _ = userColl.UpdateOne(r.Context(), bson.M{"_id": uObjID}, bson.M{"$set": updateFields})

	auditColl := config.DB.Collection("auditlogs")
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:         primitive.NewObjectID(),
		Admin:      adminUser.ID,
		Action:     "USER_PROFILE_UPDATED",
		TargetUser: &uObjID,
		Details:    fmt.Sprintf("Updated profile for %s", user.Email),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"_id":    user.ID,
		"name":   user.Name,
		"email":  user.Email,
		"role":   user.Role,
		"status": user.Status,
	})
}

// DeleteUserAdmin deletes a user and cascades team removal
// DELETE /api/admin/users/:userId
func DeleteUserAdmin(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	uIDStr := r.PathValue("userId")
	uObjID, err := primitive.ObjectIDFromHex(uIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	if adminUser.ID == uObjID {
		utils.WriteError(w, http.StatusBadRequest, "Cannot delete your own account")
		return
	}

	userColl := config.DB.Collection("users")
	var target models.User
	if err := userColl.FindOne(r.Context(), bson.M{"_id": uObjID}).Decode(&target); err != nil {
		utils.WriteError(w, http.StatusNotFound, "User not found")
		return
	}

	teamColl := config.DB.Collection("teams")
	_, _ = teamColl.UpdateMany(r.Context(), bson.M{"members": uObjID}, bson.M{"$pull": bson.M{"members": uObjID}})
	_, _ = teamColl.DeleteMany(r.Context(), bson.M{"owner": uObjID})
	_, _ = userColl.DeleteOne(r.Context(), bson.M{"_id": uObjID})

	auditColl := config.DB.Collection("auditlogs")
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:        primitive.NewObjectID(),
		Admin:     adminUser.ID,
		Action:    "USER_DELETED_PERMANENTLY",
		Details:   fmt.Sprintf("Deleted user %s permanently", target.Email),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "User removed completely"})
}

// BulkUpdateUserStatus bulk updates status for multiple users
// PATCH /api/admin/users/bulk-status
func BulkUpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	var req struct {
		UserIDs []string `json:"userIds"`
		Status  string   `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.UserIDs) == 0 || req.Status == "" {
		utils.WriteError(w, http.StatusBadRequest, "userIds and status are required")
		return
	}

	var objIDs []primitive.ObjectID
	for _, idStr := range req.UserIDs {
		if oID, err := primitive.ObjectIDFromHex(idStr); err == nil && oID != adminUser.ID {
			objIDs = append(objIDs, oID)
		}
	}

	userColl := config.DB.Collection("users")
	result, err := userColl.UpdateMany(r.Context(), bson.M{"_id": bson.M{"$in": objIDs}}, bson.M{"$set": bson.M{"status": req.Status, "updatedAt": time.Now()}})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to update users")
		return
	}

	action := "BULK_USER_SUSPENDED"
	if req.Status == "active" {
		action = "BULK_USER_ACTIVATED"
	}
	auditColl := config.DB.Collection("auditlogs")
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:        primitive.NewObjectID(),
		Admin:     adminUser.ID,
		Action:    action,
		Details:   fmt.Sprintf("Bulk updated status to %s for %d users", req.Status, result.ModifiedCount),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"message":       fmt.Sprintf("Updated %d users", result.ModifiedCount),
		"modifiedCount": result.ModifiedCount,
	})
}

