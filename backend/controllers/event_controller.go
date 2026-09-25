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

type EventInput struct {
	Title       string `json:"title"`
	Start       string `json:"start"`
	End         string `json:"end"`
	AllDay      bool   `json:"allDay"`
	Description string `json:"description"`
	Color       string `json:"color"`
	TeamID      string `json:"teamId"`
	IsGlobal    bool   `json:"isGlobal"`
}

// GetEvents fetches events
// GET /api/events?teamId=...
func GetEvents(w http.ResponseWriter, r *http.Request) {
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
				filter["$or"] = bson.A{
					bson.M{"team": tID},
					bson.M{"isGlobal": true},
				}
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
		var userTeams []models.Team
		if err == nil {
			_ = teamCursor.All(r.Context(), &userTeams)
		}

		var teamIDs []primitive.ObjectID
		teamIDMap := make(map[string]bool)
		for _, t := range userTeams {
			teamIDs = append(teamIDs, t.ID)
			teamIDMap[t.ID.Hex()] = true
		}

		if teamIDQuery != "" && teamIDQuery != "all" {
			if !teamIDMap[teamIDQuery] {
				utils.WriteError(w, http.StatusUnauthorized, "Not authorized to view this team calendar")
				return
			}
			tID, _ := primitive.ObjectIDFromHex(teamIDQuery)
			filter["$or"] = bson.A{
				bson.M{"team": tID},
				bson.M{"isGlobal": true},
				bson.M{"user": user.ID, "team": nil},
			}
		} else {
			filter["$or"] = bson.A{
				bson.M{"user": user.ID},
				bson.M{"team": bson.M{"$in": teamIDs}},
				bson.M{"isGlobal": true},
			}
		}
	}

	eventColl := config.DB.Collection("events")
	opts := options.Find().SetSort(bson.M{"start": 1})
	cursor, err := eventColl.Find(r.Context(), filter, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load events")
		return
	}
	defer cursor.Close(r.Context())

	var events []models.Event
	_ = cursor.All(r.Context(), &events)

	utils.WriteJSON(w, http.StatusOK, events)
}

// CreateEvent creates an event
// POST /api/events
func CreateEvent(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req EventInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" || req.Start == "" || req.End == "" {
		utils.WriteError(w, http.StatusBadRequest, "Please provide title, start, and end dates")
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid start date format")
		return
	}
	endTime, err := time.Parse(time.RFC3339, req.End)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid end date format")
		return
	}

	var teamObjID *primitive.ObjectID
	if req.TeamID != "" {
		if tID, err := primitive.ObjectIDFromHex(req.TeamID); err == nil {
			teamObjID = &tID
		}
	}

	isGlobal := false
	if user.Role == models.RoleMasterAdmin {
		isGlobal = req.IsGlobal
	}

	color := "#3b82f6"
	if req.Color != "" {
		color = req.Color
	}

	now := time.Now()
	newEvent := models.Event{
		ID:          primitive.NewObjectID(),
		Title:       req.Title,
		Start:       startTime,
		End:         endTime,
		AllDay:      req.AllDay,
		User:        user.ID,
		Team:        teamObjID,
		IsGlobal:    isGlobal,
		Description: req.Description,
		Color:       color,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	eventColl := config.DB.Collection("events")
	_, err = eventColl.InsertOne(r.Context(), newEvent)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create event")
		return
	}

	if teamObjID != nil {
		teamColl := config.DB.Collection("teams")
		var team models.Team
		if err := teamColl.FindOne(r.Context(), bson.M{"_id": teamObjID}).Decode(&team); err == nil {
			var recIDs []string
			if team.Owner != user.ID {
				recIDs = append(recIDs, team.Owner.Hex())
			}
			for _, m := range team.Members {
				if m != user.ID {
					recIDs = append(recIDs, m.Hex())
				}
			}
			_, _ = utils.CreateNotifications(r.Context(), recIDs, utils.NotificationInput{
				Sender:      &user.ID,
				Title:       "New Event Scheduled",
				Description: fmt.Sprintf("A new event \"%s\" has been scheduled for %s", req.Title, startTime.Format("Jan 02, 2006")),
				Type:        "event_created",
				Link:        "/calendar",
			})
			utils.EmitTeamUpdate(teamObjID.Hex(), "EVENT_CREATE", nil)
		}
	} else if isGlobal {
		utils.EmitTeamUpdate("", "PLATFORM_DATA_UPDATE", nil)
	}

	utils.WriteJSON(w, http.StatusCreated, newEvent)
}

// UpdateEvent updates an event
// PUT /api/events/:id
func UpdateEvent(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	idStr := r.PathValue("id")
	eventObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	eventColl := config.DB.Collection("events")
	var existing models.Event
	if err := eventColl.FindOne(r.Context(), bson.M{"_id": eventObjID}).Decode(&existing); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Event not found")
		return
	}

	if existing.User != user.ID && user.Role != models.RoleMasterAdmin {
		utils.WriteError(w, http.StatusUnauthorized, "User not authorized")
		return
	}

	var req EventInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	updateFields := bson.M{
		"updatedAt": time.Now(),
	}
	if req.Title != "" {
		updateFields["title"] = req.Title
	}
	if req.Start != "" {
		if st, err := time.Parse(time.RFC3339, req.Start); err == nil {
			updateFields["start"] = st
		}
	}
	if req.End != "" {
		if et, err := time.Parse(time.RFC3339, req.End); err == nil {
			updateFields["end"] = et
		}
	}
	if req.Description != "" {
		updateFields["description"] = req.Description
	}
	if req.Color != "" {
		updateFields["color"] = req.Color
	}
	updateFields["allDay"] = req.AllDay

	if user.Role == models.RoleMasterAdmin {
		updateFields["isGlobal"] = req.IsGlobal
	}

	_, _ = eventColl.UpdateOne(r.Context(), bson.M{"_id": eventObjID}, bson.M{"$set": updateFields})

	var updated models.Event
	_ = eventColl.FindOne(r.Context(), bson.M{"_id": eventObjID}).Decode(&updated)

	if updated.Team != nil {
		utils.EmitTeamUpdate(updated.Team.Hex(), "EVENT_UPDATE", nil)
	} else if updated.IsGlobal {
		utils.EmitTeamUpdate("", "PLATFORM_DATA_UPDATE", nil)
	}

	utils.WriteJSON(w, http.StatusOK, updated)
}

// DeleteEvent deletes an event
// DELETE /api/events/:id
func DeleteEvent(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	idStr := r.PathValue("id")
	eventObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid event ID")
		return
	}

	eventColl := config.DB.Collection("events")
	var existing models.Event
	if err := eventColl.FindOne(r.Context(), bson.M{"_id": eventObjID}).Decode(&existing); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Event not found")
		return
	}

	if existing.User != user.ID && user.Role != models.RoleMasterAdmin {
		utils.WriteError(w, http.StatusUnauthorized, "User not authorized")
		return
	}

	_, _ = eventColl.DeleteOne(r.Context(), bson.M{"_id": eventObjID})

	if existing.Team != nil {
		utils.EmitTeamUpdate(existing.Team.Hex(), "EVENT_DELETE", nil)
	} else if existing.IsGlobal {
		utils.EmitTeamUpdate("", "PLATFORM_DATA_UPDATE", nil)
	}

	utils.WriteJSON(w, http.StatusOK, map[string]any{"id": idStr})
}
