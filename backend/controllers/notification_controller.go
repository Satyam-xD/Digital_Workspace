package controllers

import (
	"math"
	"net/http"
	"strconv"

	"backend/config"
	"backend/middleware"
	"backend/models"
	"backend/socket"
	"backend/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetUnreadCount returns unread notifications count for header badge
// GET /api/notifications/count
func GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	notifColl := config.DB.Collection("notifications")
	count, _ := notifColl.CountDocuments(r.Context(), bson.M{"recipient": user.ID, "read": false})

	utils.WriteJSON(w, http.StatusOK, map[string]int64{"count": count})
}

// GetNotifications returns user's notifications
// GET /api/notifications
func GetNotifications(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 20
	}
	skip := int64((page - 1) * limit)

	filter := bson.M{"recipient": user.ID}
	if readQuery := r.URL.Query().Get("read"); readQuery != "" {
		filter["read"] = readQuery == "true"
	}
	if typeQuery := r.URL.Query().Get("type"); typeQuery != "" {
		filter["type"] = typeQuery
	}

	notifColl := config.DB.Collection("notifications")
	total, _ := notifColl.CountDocuments(r.Context(), filter)

	opts := options.Find().
		SetSort(bson.M{"createdAt": -1}).
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := notifColl.Find(r.Context(), filter, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load notifications")
		return
	}
	defer cursor.Close(r.Context())

	var notifications []models.Notification
	_ = cursor.All(r.Context(), &notifications)

	userColl := config.DB.Collection("users")
	for i := range notifications {
		if notifications[i].Sender != nil {
			var s models.UserSummary
			if err := userColl.FindOne(r.Context(), bson.M{"_id": notifications[i].Sender}).Decode(&s); err == nil {
				notifications[i].SenderDetails = &s
			}
		}
	}

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"notifications": notifications,
		"page":          page,
		"pages":         int(math.Ceil(float64(total) / float64(limit))),
		"total":         total,
	})
}

// MarkAsRead marks a notification as read
// PUT /api/notifications/:id/read
func MarkAsRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	idStr := r.PathValue("id")
	notifObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	notifColl := config.DB.Collection("notifications")
	_, err = notifColl.UpdateOne(r.Context(), bson.M{"_id": notifObjID, "recipient": user.ID}, bson.M{"$set": bson.M{"read": true}})
	if err != nil {
		utils.WriteError(w, http.StatusNotFound, "Notification not found or unauthorized")
		return
	}

	socket.BroadcastToRoom(user.ID.Hex(), "notificationStatusSync", map[string]any{
		"id":   idStr,
		"read": true,
	})

	var updated models.Notification
	_ = notifColl.FindOne(r.Context(), bson.M{"_id": notifObjID}).Decode(&updated)

	utils.WriteJSON(w, http.StatusOK, updated)
}

// MarkAllAsRead marks all notifications as read
// PUT /api/notifications/read-all
func MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	notifColl := config.DB.Collection("notifications")
	_, _ = notifColl.UpdateMany(r.Context(), bson.M{"recipient": user.ID, "read": false}, bson.M{"$set": bson.M{"read": true}})

	socket.BroadcastToRoom(user.ID.Hex(), "notificationStatusSync", map[string]any{
		"allRead": true,
	})

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "All notifications marked as read"})
}

// DeleteNotification removes a notification
// DELETE /api/notifications/:id
func DeleteNotification(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	idStr := r.PathValue("id")
	notifObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	notifColl := config.DB.Collection("notifications")
	_, _ = notifColl.DeleteOne(r.Context(), bson.M{"_id": notifObjID, "recipient": user.ID})

	utils.WriteJSON(w, http.StatusOK, map[string]string{"id": idStr})
}

// ClearAllNotifications removes all notifications for a user
// DELETE /api/notifications/clear-all
func ClearAllNotifications(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	notifColl := config.DB.Collection("notifications")
	_, _ = notifColl.DeleteMany(r.Context(), bson.M{"recipient": user.ID})

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "All notifications cleared"})
}

// ClearReadNotifications removes all read notifications for a user
// DELETE /api/notifications/clear-read
func ClearReadNotifications(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	notifColl := config.DB.Collection("notifications")
	_, _ = notifColl.DeleteMany(r.Context(), bson.M{"recipient": user.ID, "read": true})

	utils.WriteJSON(w, http.StatusOK, map[string]string{"message": "Read notifications cleared"})
}
