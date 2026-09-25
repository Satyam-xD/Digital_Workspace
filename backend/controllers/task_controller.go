package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"backend/config"
	"backend/middleware"
	"backend/models"
	"backend/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TaskInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	Tag         string `json:"tag"`
	TeamID      string `json:"teamId"`
	AssignedTo  string `json:"assignedTo"`
	DueDate     string `json:"dueDate"`
}

// GetTasks fetches tasks
// GET /api/tasks?teamId=...
func GetTasks(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	teamIDQuery := r.URL.Query().Get("teamId")
	filter := bson.M{}

	if user.Role == models.RoleMasterAdmin {
		if teamIDQuery != "" && teamIDQuery != "all" {
			tID, err := primitive.ObjectIDFromHex(teamIDQuery)
			if err == nil {
				filter["team"] = tID
			}
		}
	} else {
		teamColl := config.DB.Collection("teams")
		teamCursor, err := teamColl.Find(r.Context(), bson.M{
			"$or": bson.A{
				bson.M{"owner": user.ID},
				bson.M{"members": user.ID},
			},
		})
		if err != nil {
			utils.WriteError(w, http.StatusInternalServerError, "Failed to load teams")
			return
		}
		var userTeams []models.Team
		_ = teamCursor.All(r.Context(), &userTeams)

		teamIDs := []primitive.ObjectID{}
		teamIDMap := make(map[string]bool)
		for _, t := range userTeams {
			teamIDs = append(teamIDs, t.ID)
			teamIDMap[t.ID.Hex()] = true
		}

		if teamIDQuery != "" && teamIDQuery != "all" {
			if !teamIDMap[teamIDQuery] {
				utils.WriteError(w, http.StatusUnauthorized, "Not authorized to view this team board")
				return
			}
			tID, _ := primitive.ObjectIDFromHex(teamIDQuery)
			filter["team"] = tID
		} else {
			if len(teamIDs) > 0 {
				filter["$or"] = bson.A{
					bson.M{"team": bson.M{"$in": teamIDs}},
					bson.M{"user": user.ID},
				}
			} else {
				filter["user"] = user.ID
			}
		}
	}

	taskColl := config.DB.Collection("tasks")
	opts := options.Find().SetSort(bson.M{"createdAt": -1})
	cursor, err := taskColl.Find(r.Context(), filter, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to fetch tasks")
		return
	}
	defer cursor.Close(r.Context())

	var tasks []models.Task
	if err := cursor.All(r.Context(), &tasks); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to read tasks")
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}

	// Populate user details for tasks
	userColl := config.DB.Collection("users")
	for i := range tasks {
		if tasks[i].AssignedTo != nil {
			var u models.UserSummary
			if err := userColl.FindOne(r.Context(), bson.M{"_id": tasks[i].AssignedTo}).Decode(&u); err == nil {
				tasks[i].AssignedToDetails = u
			}
		}
		if tasks[i].CompletedBy != nil {
			var u models.UserSummary
			if err := userColl.FindOne(r.Context(), bson.M{"_id": tasks[i].CompletedBy}).Decode(&u); err == nil {
				tasks[i].CompletedByDetails = u
			}
		}
		if tasks[i].LastModifiedBy != nil {
			var u models.UserSummary
			if err := userColl.FindOne(r.Context(), bson.M{"_id": tasks[i].LastModifiedBy}).Decode(&u); err == nil {
				tasks[i].LastModifiedByDetails = u
			}
		}
	}

	utils.WriteJSON(w, http.StatusOK, tasks)
}

// SetTask creates a task
// POST /api/tasks
func SetTask(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	if user.Role == models.RoleTeamMember {
		utils.WriteError(w, http.StatusForbidden, "Only Team Heads can create tasks")
		return
	}

	var input TaskInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if strings.TrimSpace(input.Title) == "" {
		utils.WriteError(w, http.StatusBadRequest, "Please add a title field")
		return
	}

	teamColl := config.DB.Collection("teams")
	var targetTeam models.Team
	var teamObjID primitive.ObjectID

	if input.TeamID != "" {
		tID, err := primitive.ObjectIDFromHex(input.TeamID)
		if err != nil {
			utils.WriteError(w, http.StatusBadRequest, "Invalid team ID")
			return
		}
		if err := teamColl.FindOne(r.Context(), bson.M{"_id": tID}).Decode(&targetTeam); err != nil {
			utils.WriteError(w, http.StatusNotFound, "Team not found")
			return
		}
		teamObjID = tID
	} else {
		if err := teamColl.FindOne(r.Context(), bson.M{"owner": user.ID}).Decode(&targetTeam); err != nil {
			utils.WriteError(w, http.StatusNotFound, "You need to create a Team first")
			return
		}
		teamObjID = targetTeam.ID
	}

	if user.Role != models.RoleMasterAdmin && targetTeam.Owner != user.ID {
		utils.WriteError(w, http.StatusForbidden, "You can only create tasks for teams you own")
		return
	}

	var assignedObjID *primitive.ObjectID
	if input.AssignedTo != "" {
		aID, err := primitive.ObjectIDFromHex(input.AssignedTo)
		if err == nil {
			assignedObjID = &aID
		}
	}

	status := "To Do"
	if input.Status != "" {
		status = input.Status
	}

	priority := "medium"
	if input.Priority != "" {
		priority = input.Priority
	}

	tag := "General"
	if input.Tag != "" {
		tag = input.Tag
	}

	now := time.Now()
	var due *time.Time
	if input.DueDate != "" {
		if parsed, err := time.Parse(time.RFC3339, input.DueDate); err == nil {
			due = &parsed
		}
	}

	newTask := models.Task{
		ID:             primitive.NewObjectID(),
		User:           user.ID,
		Team:           &teamObjID,
		AssignedTo:     assignedObjID,
		Title:          input.Title,
		Description:    input.Description,
		Status:         status,
		Priority:       priority,
		Tag:            tag,
		DueDate:        due,
		LastModifiedBy: &user.ID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	taskColl := config.DB.Collection("tasks")
	_, err := taskColl.InsertOne(r.Context(), newTask)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create task")
		return
	}

	// Real-time update
	utils.EmitTeamUpdate(teamObjID.Hex(), "TASK_CREATE", nil)

	// Notify team members
	var recipientIDs []string
	if targetTeam.Owner != user.ID {
		recipientIDs = append(recipientIDs, targetTeam.Owner.Hex())
	}
	for _, m := range targetTeam.Members {
		if m != user.ID {
			recipientIDs = append(recipientIDs, m.Hex())
		}
	}

	notifTitle := "New Task Created"
	if assignedObjID != nil {
		notifTitle = "New Task Assigned"
	}

	_, _ = utils.CreateNotifications(r.Context(), recipientIDs, utils.NotificationInput{
		Sender:      &user.ID,
		Title:       notifTitle,
		Description: fmt.Sprintf("%s created task: %s", user.Name, newTask.Title),
		Type:        "task_assigned",
		Link:        "/tasks",
	})

	// Log activity
	actColl := config.DB.Collection("activities")
	_, _ = actColl.InsertOne(r.Context(), models.Activity{
		ID:        primitive.NewObjectID(),
		TeamOwner: targetTeam.Owner,
		Team:      &teamObjID,
		Text:      fmt.Sprintf("%s created task: %s", user.Name, newTask.Title),
		Type:      "task_create",
		CreatedAt: now,
		UpdatedAt: now,
	})

	utils.WriteJSON(w, http.StatusCreated, newTask)
}

// UpdateTask updates a task
// PUT /api/tasks/:id
func UpdateTask(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	idStr := r.PathValue("id")
	taskObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	taskColl := config.DB.Collection("tasks")
	var task models.Task
	if err := taskColl.FindOne(r.Context(), bson.M{"_id": taskObjID}).Decode(&task); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Task not found")
		return
	}

	var input TaskInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	updateFields := bson.M{
		"updatedAt":      time.Now(),
		"lastModifiedBy": user.ID,
	}

	if input.Title != "" {
		updateFields["title"] = input.Title
	}
	if input.Description != "" {
		updateFields["description"] = input.Description
	}
	if input.Priority != "" {
		updateFields["priority"] = input.Priority
	}
	if input.Tag != "" {
		updateFields["tag"] = input.Tag
	}
	if input.AssignedTo != "" {
		aID, err := primitive.ObjectIDFromHex(input.AssignedTo)
		if err == nil {
			updateFields["assignedTo"] = aID
		}
	}

	if input.Status != "" {
		updateFields["status"] = input.Status
		if input.Status == "Done" {
			now := time.Now()
			updateFields["completedBy"] = user.ID
			updateFields["completedAt"] = now
		}
	}

	_, err = taskColl.UpdateOne(r.Context(), bson.M{"_id": taskObjID}, bson.M{"$set": updateFields})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to update task")
		return
	}

	var updatedTask models.Task
	_ = taskColl.FindOne(r.Context(), bson.M{"_id": taskObjID}).Decode(&updatedTask)

	if task.Team != nil {
		utils.EmitTeamUpdate(task.Team.Hex(), "TASK_UPDATE", nil)
	}

	utils.WriteJSON(w, http.StatusOK, updatedTask)
}

// DeleteTask removes a task
// DELETE /api/tasks/:id
func DeleteTask(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	idStr := r.PathValue("id")
	taskObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid task ID")
		return
	}

	taskColl := config.DB.Collection("tasks")
	var task models.Task
	if err := taskColl.FindOne(r.Context(), bson.M{"_id": taskObjID}).Decode(&task); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Task not found")
		return
	}

	_, err = taskColl.DeleteOne(r.Context(), bson.M{"_id": taskObjID})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to delete task")
		return
	}

	if task.Team != nil {
		utils.EmitTeamUpdate(task.Team.Hex(), "TASK_DELETE", nil)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]any{"id": idStr})
}
