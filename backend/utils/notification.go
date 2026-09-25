package utils

import (
	"context"
	"log"
	"time"

	"backend/config"
	"backend/models"
	"backend/socket"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type NotificationInput struct {
	Sender      *primitive.ObjectID `json:"sender,omitempty"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Type        string              `json:"type"`
	Link        string              `json:"link,omitempty"`
}

type TeamUpdatePayload struct {
	TeamID     string `json:"teamId"`
	UpdateType string `json:"updateType"`
	Timestamp  string `json:"timestamp"`
}

// CreateNotifications creates notification records in bulk and emits real-time updates via Socket.IO.
func CreateNotifications(ctx context.Context, recipientIDs []string, data NotificationInput) ([]models.Notification, error) {
	if config.DB == nil {
		return nil, nil
	}

	// 1. Fetch master admins
	userColl := config.DB.Collection("users")
	cursor, err := userColl.Find(ctx, bson.M{"role": models.RoleMasterAdmin})
	var masterAdmins []models.User
	if err == nil {
		_ = cursor.All(ctx, &masterAdmins)
	}

	// 2. Deduplicate recipients + master admins
	recipientSet := make(map[string]bool)
	for _, id := range recipientIDs {
		if id != "" {
			recipientSet[id] = true
		}
	}
	for _, admin := range masterAdmins {
		recipientSet[admin.ID.Hex()] = true
	}

	// 3. Filter out sender
	if data.Sender != nil {
		delete(recipientSet, data.Sender.Hex())
	}

	if len(recipientSet) == 0 {
		return []models.Notification{}, nil
	}

	// 4. Build notifications
	now := time.Now()
	var docs []any
	var createdList []models.Notification

	for recIDStr := range recipientSet {
		recObjID, err := primitive.ObjectIDFromHex(recIDStr)
		if err != nil {
			continue
		}
		notif := models.Notification{
			ID:          primitive.NewObjectID(),
			Recipient:   recObjID,
			Sender:      data.Sender,
			Title:       data.Title,
			Description: data.Description,
			Type:        data.Type,
			Link:        data.Link,
			Read:        false,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		docs = append(docs, notif)
		createdList = append(createdList, notif)
	}

	if len(docs) == 0 {
		return []models.Notification{}, nil
	}

	notifColl := config.DB.Collection("notifications")
	_, err = notifColl.InsertMany(ctx, docs)
	if err != nil {
		log.Printf("[Notification] InsertMany error: %v", err)
		return nil, err
	}

	// 5. Emit real-time updates
	for _, notif := range createdList {
		socket.BroadcastToRoom(notif.Recipient.Hex(), "newNotification", notif)
	}

	return createdList, nil
}

// CreateNotification sends a single notification
func CreateNotification(ctx context.Context, recipientID string, data NotificationInput) ([]models.Notification, error) {
	return CreateNotifications(ctx, []string{recipientID}, data)
}

// EmitTeamUpdate emits a real-time update event to members of a team or to the platform
func EmitTeamUpdate(teamID string, updateType string, recipientIDs []string) {
	payload := TeamUpdatePayload{
		TeamID:     teamID,
		UpdateType: updateType,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	if len(recipientIDs) > 0 {
		for _, id := range recipientIDs {
			socket.BroadcastToRoom(id, "team_update", payload)
		}
	} else if teamID != "" {
		socket.BroadcastToRoom("team_"+teamID, "team_update", payload)
		socket.BroadcastToRoom("platform_admin", "team_update", payload)
	} else {
		socket.BroadcastGlobal("platform_update", payload)
	}
}
