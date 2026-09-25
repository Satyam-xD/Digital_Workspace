package controllers

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

type TeamDetailsInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TeamID      string `json:"teamId,omitempty"`
}

type AddMemberInput struct {
	Email  string `json:"email"`
	TeamID string `json:"teamId,omitempty"`
}

type CreateTeamInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type EnrichedMember struct {
	ID             primitive.ObjectID `json:"_id"`
	Name           string             `json:"name"`
	Email          string             `json:"email"`
	Role           string             `json:"role"`
	TasksAssigned  int                `json:"tasksAssigned"`
	TasksCompleted int                `json:"tasksCompleted"`
	IsOwner        bool               `json:"isOwner,omitempty"`
}

type EnrichedTeam struct {
	ID          primitive.ObjectID `json:"_id"`
	Id          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Owner       *EnrichedMember    `json:"owner,omitempty"`
	Members     []EnrichedMember   `json:"members"`
	IsOwner     bool               `json:"isOwner"`
	OwnerName   string             `json:"ownerName"`
	CreatedAt   time.Time          `json:"createdAt,omitempty"`
	UpdatedAt   time.Time          `json:"updatedAt,omitempty"`
}

// GetTeamMembers fetches teams and member statistics
// GET /api/team
func GetTeamMembers(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	teamColl := config.DB.Collection("teams")
	userColl := config.DB.Collection("users")
	taskColl := config.DB.Collection("tasks")

	isMaster := user.Role == models.RoleMasterAdmin

	// Check if user is team_head/admin with 0 owned teams -> auto create default team
	if !isMaster && (user.Role == models.RoleTeamHead || user.Role == models.RoleAdmin) {
		ownedCount, _ := teamColl.CountDocuments(r.Context(), bson.M{"owner": user.ID})
		if ownedCount == 0 {
			var u models.User
			_ = userColl.FindOne(r.Context(), bson.M{"_id": user.ID}).Decode(&u)
			teamName := u.TeamName
			if teamName == "" {
				teamName = "My Team"
			}
			teamDesc := u.TeamDescription
			if teamDesc == "" {
				teamDesc = "Team managed by you"
			}
			members := u.TeamMembers
			if members == nil {
				members = []primitive.ObjectID{}
			}
			defaultTeam := models.Team{
				ID:          primitive.NewObjectID(),
				Name:        teamName,
				Description: teamDesc,
				Owner:       user.ID,
				Members:     members,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			_, _ = teamColl.InsertOne(r.Context(), defaultTeam)
		}
	}

	filter := bson.M{}
	if !isMaster {
		filter = bson.M{
			"$or": bson.A{
				bson.M{"owner": user.ID},
				bson.M{"members": user.ID},
			},
		}
	}

	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := teamColl.Find(r.Context(), filter, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load teams")
		return
	}
	defer cursor.Close(r.Context())

	var teams []models.Team
	_ = cursor.All(r.Context(), &teams)

	// Helper for user task stats
	getMemberStats := func(uid primitive.ObjectID) (int, int) {
		assigned, _ := taskColl.CountDocuments(r.Context(), bson.M{"assignedTo": uid})
		completed, _ := taskColl.CountDocuments(r.Context(), bson.M{"assignedTo": uid, "status": "Done"})
		return int(assigned), int(completed)
	}

	var result []EnrichedTeam
	for _, t := range teams {
		var ownerUser models.User
		_ = userColl.FindOne(r.Context(), bson.M{"_id": t.Owner}).Decode(&ownerUser)
		ownerAssigned, ownerCompleted := getMemberStats(t.Owner)

		ownerEnriched := EnrichedMember{
			ID:             ownerUser.ID,
			Name:           ownerUser.Name,
			Email:          ownerUser.Email,
			Role:           string(ownerUser.Role),
			TasksAssigned:  ownerAssigned,
			TasksCompleted: ownerCompleted,
			IsOwner:        true,
		}

		var membersEnriched []EnrichedMember
		// Include owner in members list if not already present (frontend Expects owner in members)
		membersEnriched = append(membersEnriched, ownerEnriched)

		for _, mID := range t.Members {
			if mID == t.Owner {
				continue
			}
			var mUser models.User
			if err := userColl.FindOne(r.Context(), bson.M{"_id": mID}).Decode(&mUser); err == nil {
				mAss, mComp := getMemberStats(mID)
				membersEnriched = append(membersEnriched, EnrichedMember{
					ID:             mUser.ID,
					Name:           mUser.Name,
					Email:          mUser.Email,
					Role:           string(mUser.Role),
					TasksAssigned:  mAss,
					TasksCompleted: mComp,
					IsOwner:        false,
				})
			}
		}

		isOwner := t.Owner == user.ID || isMaster
		ownerName := ownerUser.Name
		if isOwner {
			ownerName = "You"
		} else if ownerName == "" {
			ownerName = "Unknown"
		}

		result = append(result, EnrichedTeam{
			ID:          t.ID,
			Id:          t.ID.Hex(),
			Name:        t.Name,
			Description: t.Description,
			Owner:       &ownerEnriched,
			Members:     membersEnriched,
			IsOwner:     isOwner,
			OwnerName:   ownerName,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
		})
	}

	utils.WriteJSON(w, http.StatusOK, result)
}

// CreateNewTeam creates a new team
// POST /api/team/create
func CreateNewTeam(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req CreateTeamInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		utils.WriteError(w, http.StatusBadRequest, "Team name is required")
		return
	}

	now := time.Now()
	team := models.Team{
		ID:          primitive.NewObjectID(),
		Name:        req.Name,
		Description: req.Description,
		Owner:       user.ID,
		Members:     []primitive.ObjectID{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	teamColl := config.DB.Collection("teams")
	_, err := teamColl.InsertOne(r.Context(), team)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create team")
		return
	}

	utils.EmitTeamUpdate(team.ID.Hex(), "TEAM_CREATE", nil)

	utils.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":          team.ID.Hex(),
		"_id":         team.ID,
		"name":        team.Name,
		"description": team.Description,
		"owner":       team.Owner,
		"members":     team.Members,
		"isOwner":     true,
		"ownerName":   "You",
	})
}

// AddTeamMember adds a user to a team and returns the updated member list
// POST /api/team and POST /api/team/members
func AddTeamMember(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req AddMemberInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		utils.WriteError(w, http.StatusBadRequest, "Please provide member email")
		return
	}

	isMaster := user.Role == models.RoleMasterAdmin
	teamColl := config.DB.Collection("teams")
	userColl := config.DB.Collection("users")

	var targetTeam models.Team
	if req.TeamID != "" {
		tID, err := primitive.ObjectIDFromHex(req.TeamID)
		if err != nil {
			utils.WriteError(w, http.StatusBadRequest, "Invalid team ID")
			return
		}
		if isMaster {
			_ = teamColl.FindOne(r.Context(), bson.M{"_id": tID}).Decode(&targetTeam)
		} else {
			_ = teamColl.FindOne(r.Context(), bson.M{"_id": tID, "owner": user.ID}).Decode(&targetTeam)
		}
	} else {
		_ = teamColl.FindOne(r.Context(), bson.M{"owner": user.ID}).Decode(&targetTeam)
	}

	if targetTeam.ID.IsZero() && req.TeamID == "" {
		// Auto-create team for user
		targetTeam = models.Team{
			ID:          primitive.NewObjectID(),
			Name:        "My Team",
			Description: "Team managed by you",
			Owner:       user.ID,
			Members:     []primitive.ObjectID{},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		_, _ = teamColl.InsertOne(r.Context(), targetTeam)
	}

	if targetTeam.ID.IsZero() {
		utils.WriteError(w, http.StatusNotFound, "Team not found or you are not the owner")
		return
	}

	var userToAdd models.User
	if err := userColl.FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&userToAdd); err != nil {
		utils.WriteError(w, http.StatusNotFound, "User not found. They must register first.")
		return
	}

	if userToAdd.ID == user.ID {
		utils.WriteError(w, http.StatusBadRequest, "You cannot add yourself")
		return
	}

	for _, m := range targetTeam.Members {
		if m == userToAdd.ID {
			utils.WriteError(w, http.StatusBadRequest, "User already in team")
			return
		}
	}

	_, _ = teamColl.UpdateOne(r.Context(), bson.M{"_id": targetTeam.ID}, bson.M{"$push": bson.M{"members": userToAdd.ID}})
	targetTeam.Members = append(targetTeam.Members, userToAdd.ID)

	utils.EmitTeamUpdate(targetTeam.ID.Hex(), "MEMBER_ADD", nil)

	// Log activity
	actColl := config.DB.Collection("activities")
	_, _ = actColl.InsertOne(r.Context(), models.Activity{
		ID:        primitive.NewObjectID(),
		TeamOwner: user.ID,
		Team:      &targetTeam.ID,
		Text:      fmt.Sprintf("Added %s to team %s", userToAdd.Name, targetTeam.Name),
		Type:      "member_add",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	// Dispatch notification
	var recipientIDs []string
	for _, m := range targetTeam.Members {
		if m != user.ID {
			recipientIDs = append(recipientIDs, m.Hex())
		}
	}
	if targetTeam.Owner != user.ID {
		recipientIDs = append(recipientIDs, targetTeam.Owner.Hex())
	}
	if len(recipientIDs) > 0 {
		_, _ = utils.CreateNotifications(r.Context(), recipientIDs, utils.NotificationInput{
			Title:       "New Team Member",
			Description: fmt.Sprintf("%s has joined the team \"%s\"", userToAdd.Name, targetTeam.Name),
			Type:        "team_update",
			Sender:      &user.ID,
			Link:        "/team",
		})
	}

	// Return updated members array expected by frontend
	taskColl := config.DB.Collection("tasks")
	getMemberStats := func(uid primitive.ObjectID) (int, int) {
		assigned, _ := taskColl.CountDocuments(r.Context(), bson.M{"assignedTo": uid})
		completed, _ := taskColl.CountDocuments(r.Context(), bson.M{"assignedTo": uid, "status": "Done"})
		return int(assigned), int(completed)
	}

	var ownerUser models.User
	_ = userColl.FindOne(r.Context(), bson.M{"_id": targetTeam.Owner}).Decode(&ownerUser)
	ownerAssigned, ownerCompleted := getMemberStats(targetTeam.Owner)

	updatedMembers := []EnrichedMember{
		{
			ID:             ownerUser.ID,
			Name:           ownerUser.Name,
			Email:          ownerUser.Email,
			Role:           string(ownerUser.Role),
			TasksAssigned:  ownerAssigned,
			TasksCompleted: ownerCompleted,
			IsOwner:        true,
		},
	}

	for _, mID := range targetTeam.Members {
		if mID == targetTeam.Owner {
			continue
		}
		var mUser models.User
		if err := userColl.FindOne(r.Context(), bson.M{"_id": mID}).Decode(&mUser); err == nil {
			ass, comp := getMemberStats(mID)
			updatedMembers = append(updatedMembers, EnrichedMember{
				ID:             mUser.ID,
				Name:           mUser.Name,
				Email:          mUser.Email,
				Role:           string(mUser.Role),
				TasksAssigned:  ass,
				TasksCompleted: comp,
				IsOwner:        false,
			})
		}
	}

	utils.WriteJSON(w, http.StatusOK, updatedMembers)
}

// RemoveTeamMember removes a user from a team
// DELETE /api/team/:teamId/member/:memberId and DELETE /api/team/members/:id
func RemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	teamIDStr := r.PathValue("teamId")
	memberIDStr := r.PathValue("memberId")
	if memberIDStr == "" {
		memberIDStr = r.PathValue("id")
	}

	memberObjID, err := primitive.ObjectIDFromHex(memberIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid member ID")
		return
	}

	isMaster := user.Role == models.RoleMasterAdmin
	teamColl := config.DB.Collection("teams")

	var team models.Team
	if teamIDStr != "" {
		tID, err := primitive.ObjectIDFromHex(teamIDStr)
		if err != nil {
			utils.WriteError(w, http.StatusBadRequest, "Invalid team ID")
			return
		}
		if isMaster {
			_ = teamColl.FindOne(r.Context(), bson.M{"_id": tID}).Decode(&team)
		} else {
			_ = teamColl.FindOne(r.Context(), bson.M{"_id": tID, "owner": user.ID}).Decode(&team)
		}
	} else {
		_ = teamColl.FindOne(r.Context(), bson.M{"owner": user.ID}).Decode(&team)
	}

	if team.ID.IsZero() {
		utils.WriteError(w, http.StatusNotFound, "Team not found")
		return
	}

	_, _ = teamColl.UpdateOne(r.Context(), bson.M{"_id": team.ID}, bson.M{"$pull": bson.M{"members": memberObjID}})
	utils.EmitTeamUpdate(team.ID.Hex(), "MEMBER_REMOVE", nil)

	// Log activity
	actColl := config.DB.Collection("activities")
	_, _ = actColl.InsertOne(r.Context(), models.Activity{
		ID:        primitive.NewObjectID(),
		TeamOwner: user.ID,
		Team:      &team.ID,
		Text:      fmt.Sprintf("Removed member from team %s", team.Name),
		Type:      "member_remove",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"id":      memberIDStr,
		"teamId":  team.ID.Hex(),
		"message": "Member removed successfully",
	})
}

// UpdateTeamDetails updates team name and description
// PUT /api/team and PUT /api/team/details
func UpdateTeamDetails(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req TeamDetailsInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	isMaster := user.Role == models.RoleMasterAdmin
	teamColl := config.DB.Collection("teams")

	var team models.Team
	if req.TeamID != "" {
		tID, err := primitive.ObjectIDFromHex(req.TeamID)
		if err == nil {
			if isMaster {
				_ = teamColl.FindOne(r.Context(), bson.M{"_id": tID}).Decode(&team)
			} else {
				_ = teamColl.FindOne(r.Context(), bson.M{"_id": tID, "owner": user.ID}).Decode(&team)
			}
		}
	} else {
		_ = teamColl.FindOne(r.Context(), bson.M{"owner": user.ID}).Decode(&team)
	}

	if team.ID.IsZero() {
		utils.WriteError(w, http.StatusNotFound, "Team not found")
		return
	}

	updateFields := bson.M{"updatedAt": time.Now()}
	if req.Name != "" {
		updateFields["name"] = req.Name
		team.Name = req.Name
	}
	if req.Description != "" {
		updateFields["description"] = req.Description
		team.Description = req.Description
	}

	_, err := teamColl.UpdateOne(r.Context(), bson.M{"_id": team.ID}, bson.M{"$set": updateFields})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to update team")
		return
	}

	utils.EmitTeamUpdate(team.ID.Hex(), "TEAM_METADATA_UPDATE", nil)

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"id":              team.ID.Hex(),
		"_id":             team.ID,
		"name":            team.Name,
		"description":     team.Description,
		"teamName":        team.Name,
		"teamDescription": team.Description,
	})
}

// DeleteTeam deletes a team and its activities
// DELETE /api/team/delete/:teamId and DELETE /api/team/:teamId
func DeleteTeam(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	teamIDStr := r.PathValue("teamId")
	teamObjID, err := primitive.ObjectIDFromHex(teamIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	isMaster := user.Role == models.RoleMasterAdmin
	teamColl := config.DB.Collection("teams")

	var team models.Team
	if isMaster {
		err = teamColl.FindOne(r.Context(), bson.M{"_id": teamObjID}).Decode(&team)
	} else {
		err = teamColl.FindOne(r.Context(), bson.M{"_id": teamObjID, "owner": user.ID}).Decode(&team)
	}
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Team not found")
		return
	}

	_, _ = teamColl.DeleteOne(r.Context(), bson.M{"_id": teamObjID})
	actColl := config.DB.Collection("activities")
	_, _ = actColl.DeleteMany(r.Context(), bson.M{"team": teamObjID})

	utils.EmitTeamUpdate(teamObjID.Hex(), "TEAM_DELETE", nil)

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"id":      teamIDStr,
		"message": "Team deleted",
	})
}

// GetTeamActivity returns recent activities for a team
// GET /api/team/activity/:teamId and GET /api/team/activities
func GetTeamActivity(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	teamIDStr := r.PathValue("teamId")
	filter := bson.M{}

	if teamIDStr != "" {
		tID, err := primitive.ObjectIDFromHex(teamIDStr)
		if err == nil {
			filter = bson.M{
				"$or": bson.A{
					bson.M{"team": tID},
					bson.M{"teamOwner": user.ID},
				},
			}
		}
	}

	actColl := config.DB.Collection("activities")
	opts := options.Find().SetSort(bson.M{"createdAt": -1}).SetLimit(50)
	cursor, err := actColl.Find(r.Context(), filter, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to fetch activities")
		return
	}
	defer cursor.Close(r.Context())

	var activities []models.Activity
	_ = cursor.All(r.Context(), &activities)

	utils.WriteJSON(w, http.StatusOK, activities)
}

// GetTeamActivities alias
func GetTeamActivities(w http.ResponseWriter, r *http.Request) {
	GetTeamActivity(w, r)
}
