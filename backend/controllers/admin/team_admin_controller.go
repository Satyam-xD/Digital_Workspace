package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"backend/config"
	"backend/middleware"
	"backend/models"
	"backend/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MemberWithStats struct {
	ID             primitive.ObjectID `json:"_id"`
	Name           string             `json:"name"`
	Email          string             `json:"email"`
	Role           string             `json:"role"`
	TasksAssigned  int                `json:"tasksAssigned"`
	TasksCompleted int                `json:"tasksCompleted"`
}

type TeamAdminView struct {
	ID          primitive.ObjectID `json:"_id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Owner       *MemberWithStats   `json:"owner"`
	Members     []MemberWithStats  `json:"members"`
	CreatedAt   time.Time          `json:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
}

// GetAllTeamsAdmin returns all teams for Master Admin oversight
// GET /api/admin/teams
func GetAllTeamsAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	teamColl := config.DB.Collection("teams")
	userColl := config.DB.Collection("users")
	taskColl := config.DB.Collection("tasks")

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := teamColl.Find(ctx, bson.M{}, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load teams")
		return
	}
	defer cursor.Close(ctx)

	var teams []models.Team
	_ = cursor.All(ctx, &teams)

	var results []TeamAdminView
	for _, team := range teams {
		var ownerUser models.User
		_ = userColl.FindOne(ctx, bson.M{"_id": team.Owner}).Decode(&ownerUser)

		ownerAssigned, _ := taskColl.CountDocuments(ctx, bson.M{"team": team.ID, "assignedTo": team.Owner})
		ownerCompleted, _ := taskColl.CountDocuments(ctx, bson.M{"team": team.ID, "assignedTo": team.Owner, "status": "Done"})

		ownerStats := &MemberWithStats{
			ID:             ownerUser.ID,
			Name:           ownerUser.Name,
			Email:          ownerUser.Email,
			Role:           string(ownerUser.Role),
			TasksAssigned:  int(ownerAssigned),
			TasksCompleted: int(ownerCompleted),
		}

		var membersStats []MemberWithStats
		for _, mID := range team.Members {
			var mUser models.User
			if err := userColl.FindOne(ctx, bson.M{"_id": mID}).Decode(&mUser); err == nil {
				ass, _ := taskColl.CountDocuments(ctx, bson.M{"team": team.ID, "assignedTo": mID})
				comp, _ := taskColl.CountDocuments(ctx, bson.M{"team": team.ID, "assignedTo": mID, "status": "Done"})
				membersStats = append(membersStats, MemberWithStats{
					ID:             mUser.ID,
					Name:           mUser.Name,
					Email:          mUser.Email,
					Role:           string(mUser.Role),
					TasksAssigned:  int(ass),
					TasksCompleted: int(comp),
				})
			}
		}

		results = append(results, TeamAdminView{
			ID:          team.ID,
			Name:        team.Name,
			Description: team.Description,
			Owner:       ownerStats,
			Members:     membersStats,
			CreatedAt:   team.CreatedAt,
			UpdatedAt:   team.UpdatedAt,
		})
	}

	utils.WriteJSON(w, http.StatusOK, results)
}

// DeleteTeamAdmin deletes a team
// DELETE /api/admin/teams/:teamId
func DeleteTeamAdmin(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	teamIDStr := r.PathValue("teamId")
	teamObjID, err := primitive.ObjectIDFromHex(teamIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	teamColl := config.DB.Collection("teams")
	var team models.Team
	if err := teamColl.FindOne(r.Context(), bson.M{"_id": teamObjID}).Decode(&team); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Team not found")
		return
	}

	_, _ = teamColl.DeleteOne(r.Context(), bson.M{"_id": teamObjID})
	actColl := config.DB.Collection("activities")
	_, _ = actColl.DeleteMany(r.Context(), bson.M{"team": teamObjID})

	auditColl := config.DB.Collection("auditlogs")
	now := time.Now()
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:        primitive.NewObjectID(),
		Admin:     adminUser.ID,
		Action:    "TEAM_DELETED",
		Details:   fmt.Sprintf("Deleted team %s", team.Name),
		CreatedAt: now,
		UpdatedAt: now,
	})

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "Team removed"})
}

// AddMemberToTeamAdmin adds member by email
// POST /api/admin/teams/:teamId/members
func AddMemberToTeamAdmin(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	teamIDStr := r.PathValue("teamId")
	teamObjID, err := primitive.ObjectIDFromHex(teamIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		utils.WriteError(w, http.StatusBadRequest, "Email is required")
		return
	}

	userColl := config.DB.Collection("users")
	var userToAdd models.User
	if err := userColl.FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&userToAdd); err != nil {
		utils.WriteError(w, http.StatusNotFound, "User not found")
		return
	}

	teamColl := config.DB.Collection("teams")
	var team models.Team
	if err := teamColl.FindOne(r.Context(), bson.M{"_id": teamObjID}).Decode(&team); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Team not found")
		return
	}

	for _, m := range team.Members {
		if m == userToAdd.ID {
			utils.WriteError(w, http.StatusBadRequest, "User is already a member")
			return
		}
	}

	_, _ = teamColl.UpdateOne(r.Context(), bson.M{"_id": teamObjID}, bson.M{"$push": bson.M{"members": userToAdd.ID}})

	auditColl := config.DB.Collection("auditlogs")
	now := time.Now()
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:         primitive.NewObjectID(),
		Admin:      adminUser.ID,
		Action:     "TEAM_MEMBER_ADDED",
		TargetTeam: &teamObjID,
		TargetUser: &userToAdd.ID,
		Details:    fmt.Sprintf("Added %s to %s", userToAdd.Email, team.Name),
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	_ = teamColl.FindOne(r.Context(), bson.M{"_id": teamObjID}).Decode(&team)

	taskColl := config.DB.Collection("tasks")
	var updatedMembers []MemberWithStats

	// Include owner
	var ownerUser models.User
	if err := userColl.FindOne(r.Context(), bson.M{"_id": team.Owner}).Decode(&ownerUser); err == nil {
		ass, _ := taskColl.CountDocuments(r.Context(), bson.M{"team": team.ID, "assignedTo": team.Owner})
		comp, _ := taskColl.CountDocuments(r.Context(), bson.M{"team": team.ID, "assignedTo": team.Owner, "status": "Done"})
		updatedMembers = append(updatedMembers, MemberWithStats{
			ID:             ownerUser.ID,
			Name:           ownerUser.Name,
			Email:          ownerUser.Email,
			Role:           string(ownerUser.Role),
			TasksAssigned:  int(ass),
			TasksCompleted: int(comp),
		})
	}

	for _, mID := range team.Members {
		if mID == team.Owner {
			continue
		}
		var mUser models.User
		if err := userColl.FindOne(r.Context(), bson.M{"_id": mID}).Decode(&mUser); err == nil {
			ass, _ := taskColl.CountDocuments(r.Context(), bson.M{"team": team.ID, "assignedTo": mID})
			comp, _ := taskColl.CountDocuments(r.Context(), bson.M{"team": team.ID, "assignedTo": mID, "status": "Done"})
			updatedMembers = append(updatedMembers, MemberWithStats{
				ID:             mUser.ID,
				Name:           mUser.Name,
				Email:          mUser.Email,
				Role:           string(mUser.Role),
				TasksAssigned:  int(ass),
				TasksCompleted: int(comp),
			})
		}
	}

	utils.EmitTeamUpdate(teamIDStr, "MEMBER_ADD", nil)
	utils.WriteJSON(w, http.StatusOK, updatedMembers)
}

// RemoveMemberFromTeamAdmin removes a member from a team
// DELETE /api/admin/teams/:teamId/members/:memberId
func RemoveMemberFromTeamAdmin(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	teamIDStr := r.PathValue("teamId")
	memberIDStr := r.PathValue("memberId")

	teamObjID, err := primitive.ObjectIDFromHex(teamIDStr)
	memberObjID, err2 := primitive.ObjectIDFromHex(memberIDStr)
	if err != nil || err2 != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid team or member ID")
		return
	}

	teamColl := config.DB.Collection("teams")
	_, err = teamColl.UpdateOne(r.Context(), bson.M{"_id": teamObjID}, bson.M{"$pull": bson.M{"members": memberObjID}})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to remove member")
		return
	}

	auditColl := config.DB.Collection("auditlogs")
	now := time.Now()
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:         primitive.NewObjectID(),
		Admin:      adminUser.ID,
		Action:     "TEAM_MEMBER_REMOVED",
		TargetTeam: &teamObjID,
		TargetUser: &memberObjID,
		Details:    "Removed user from team",
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	utils.EmitTeamUpdate(teamIDStr, "MEMBER_REMOVE", nil)
	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "User removed from team"})
}

// TransferTeamOwnership reassigns team owner
// PUT /api/admin/teams/:teamId/transfer-ownership
func TransferTeamOwnership(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	teamIDStr := r.PathValue("teamId")
	teamObjID, err := primitive.ObjectIDFromHex(teamIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	var req struct {
		NewOwnerID string `json:"newOwnerId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NewOwnerID == "" {
		utils.WriteError(w, http.StatusBadRequest, "newOwnerId is required")
		return
	}

	newOwnerObjID, err := primitive.ObjectIDFromHex(req.NewOwnerID)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid new owner ID")
		return
	}

	teamColl := config.DB.Collection("teams")
	_, err = teamColl.UpdateOne(r.Context(), bson.M{"_id": teamObjID}, bson.M{"$set": bson.M{"owner": newOwnerObjID}})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to transfer ownership")
		return
	}

	auditColl := config.DB.Collection("auditlogs")
	now := time.Now()
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:         primitive.NewObjectID(),
		Admin:      adminUser.ID,
		Action:     "TEAM_OWNERSHIP_TRANSFERRED",
		TargetTeam: &teamObjID,
		TargetUser: &newOwnerObjID,
		Details:    "Transferred team ownership",
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "Ownership transferred"})
}

// CreateTeamAdmin creates a team on behalf of an owner
// POST /api/admin/teams
func CreateTeamAdmin(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		OwnerID     string `json:"ownerId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.OwnerID == "" {
		utils.WriteError(w, http.StatusBadRequest, "Name and ownerId are required")
		return
	}

	ownerObjID, err := primitive.ObjectIDFromHex(req.OwnerID)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid owner ID")
		return
	}

	now := time.Now()
	newTeam := models.Team{
		ID:          primitive.NewObjectID(),
		Name:        req.Name,
		Description: req.Description,
		Owner:       ownerObjID,
		Members:     []primitive.ObjectID{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	teamColl := config.DB.Collection("teams")
	_, err = teamColl.InsertOne(r.Context(), newTeam)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create team")
		return
	}

	auditColl := config.DB.Collection("auditlogs")
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:         primitive.NewObjectID(),
		Admin:      adminUser.ID,
		Action:     "TEAM_CREATED",
		TargetTeam: &newTeam.ID,
		Details:    fmt.Sprintf("Created team %s", newTeam.Name),
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	utils.WriteJSON(w, http.StatusCreated, newTeam)
}

// UpdateTeamDetailsAdmin updates team details
// PUT /api/admin/teams/:teamId/details
func UpdateTeamDetailsAdmin(w http.ResponseWriter, r *http.Request) {
	adminUser := middleware.GetUserFromContext(r)
	teamIDStr := r.PathValue("teamId")
	teamObjID, err := primitive.ObjectIDFromHex(teamIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	updateFields := bson.M{"updatedAt": time.Now()}
	if req.Name != "" {
		updateFields["name"] = req.Name
	}
	if req.Description != "" {
		updateFields["description"] = req.Description
	}

	teamColl := config.DB.Collection("teams")
	_, err = teamColl.UpdateOne(r.Context(), bson.M{"_id": teamObjID}, bson.M{"$set": updateFields})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to update team")
		return
	}

	auditColl := config.DB.Collection("auditlogs")
	now := time.Now()
	_, _ = auditColl.InsertOne(r.Context(), models.AuditLog{
		ID:         primitive.NewObjectID(),
		Admin:      adminUser.ID,
		Action:     "TEAM_DETAILS_UPDATED",
		TargetTeam: &teamObjID,
		Details:    "Updated team details",
		CreatedAt:  now,
		UpdatedAt:  now,
	})

	var updatedTeam models.Team
	_ = teamColl.FindOne(r.Context(), bson.M{"_id": teamObjID}).Decode(&updatedTeam)

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"id":              teamObjID.Hex(),
		"_id":             teamObjID,
		"name":            updatedTeam.Name,
		"description":     updatedTeam.Description,
		"teamName":        updatedTeam.Name,
		"teamDescription": updatedTeam.Description,
	})
}
