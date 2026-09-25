package controllers

import (
	"encoding/json"
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

type PasswordInput struct {
	Title    string `json:"title"`
	Username string `json:"username"`
	Password string `json:"password"`
	URL      string `json:"url"`
	Category string `json:"category"`
	Notes    string `json:"notes"`
}

// GetPasswords returns all passwords for the authenticated user
// GET /api/passwords
func GetPasswords(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	passColl := config.DB.Collection("passwords")
	opts := options.Find().SetSort(bson.M{"updatedAt": -1})
	cursor, err := passColl.Find(r.Context(), bson.M{"user": user.ID}, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load passwords")
		return
	}
	defer cursor.Close(r.Context())

	var passwords []models.Password
	_ = cursor.All(r.Context(), &passwords)

	utils.WriteJSON(w, http.StatusOK, passwords)
}

// CreatePassword creates a password entry
// POST /api/passwords
func CreatePassword(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req PasswordInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" || req.Username == "" || req.Password == "" {
		utils.WriteError(w, http.StatusBadRequest, "Please fill in all required fields")
		return
	}

	category := "login"
	if req.Category != "" {
		category = req.Category
	}

	now := time.Now()
	newPass := models.Password{
		ID:        primitive.NewObjectID(),
		User:      user.ID,
		Title:     req.Title,
		Username:  req.Username,
		Password:  req.Password,
		URL:       req.URL,
		Category:  category,
		Notes:     req.Notes,
		CreatedAt: now,
		UpdatedAt: now,
	}

	passColl := config.DB.Collection("passwords")
	_, err := passColl.InsertOne(r.Context(), newPass)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create password entry")
		return
	}

	utils.WriteJSON(w, http.StatusCreated, newPass)
}

// UpdatePassword updates an existing password entry
// PUT /api/passwords/:id
func UpdatePassword(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	idStr := r.PathValue("id")
	passObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid password ID")
		return
	}

	var req PasswordInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	passColl := config.DB.Collection("passwords")
	var existing models.Password
	if err := passColl.FindOne(r.Context(), bson.M{"_id": passObjID}).Decode(&existing); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Password entry not found")
		return
	}

	if existing.User != user.ID {
		utils.WriteError(w, http.StatusUnauthorized, "User not authorized")
		return
	}

	updateFields := bson.M{
		"updatedAt": time.Now(),
	}
	if req.Title != "" {
		updateFields["title"] = req.Title
	}
	if req.Username != "" {
		updateFields["username"] = req.Username
	}
	if req.Password != "" {
		updateFields["password"] = req.Password
	}
	if req.URL != "" {
		updateFields["url"] = req.URL
	}
	if req.Category != "" {
		updateFields["category"] = req.Category
	}
	if req.Notes != "" {
		updateFields["notes"] = req.Notes
	}

	_, err = passColl.UpdateOne(r.Context(), bson.M{"_id": passObjID}, bson.M{"$set": updateFields})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to update password entry")
		return
	}

	var updated models.Password
	_ = passColl.FindOne(r.Context(), bson.M{"_id": passObjID}).Decode(&updated)

	utils.WriteJSON(w, http.StatusOK, updated)
}

// DeletePassword deletes a password entry
// DELETE /api/passwords/:id
func DeletePassword(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	idStr := r.PathValue("id")
	passObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid password ID")
		return
	}

	passColl := config.DB.Collection("passwords")
	var existing models.Password
	if err := passColl.FindOne(r.Context(), bson.M{"_id": passObjID}).Decode(&existing); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Password entry not found")
		return
	}

	if existing.User != user.ID {
		utils.WriteError(w, http.StatusUnauthorized, "User not authorized")
		return
	}

	_, _ = passColl.DeleteOne(r.Context(), bson.M{"_id": passObjID})
	utils.WriteJSON(w, http.StatusOK, map[string]any{"id": idStr})
}
