package controllers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"backend/config"
	"backend/middleware"
	"backend/models"
	"backend/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type AuthResponse struct {
	ID      primitive.ObjectID `json:"_id"`
	Name    string             `json:"name"`
	Email   string             `json:"email"`
	Role    models.UserRole    `json:"role"`
	Token   string             `json:"token"`
	Pending bool               `json:"pending,omitempty"`
	Message string             `json:"message,omitempty"`
}

// AuthUser handles user login
// POST /api/auth/login
func AuthUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	userColl := config.DB.Collection("users")
	var user models.User
	err := userColl.FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&user)
	if err != nil {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	if !user.MatchPassword(req.Password) {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	isValidRole := false
	if string(user.Role) == req.Role {
		isValidRole = true
	} else if user.Role == models.RoleAdmin && req.Role == "team_head" {
		isValidRole = true
	}

	if req.Role != "" && !isValidRole {
		utils.WriteError(w, http.StatusUnauthorized, "Invalid role selected.")
		return
	}

	if user.Status == models.StatusPending {
		utils.WriteError(w, http.StatusForbidden, "Your account is awaiting Master Admin approval. Please check back later.")
		return
	}

	if user.Status == models.StatusSuspended {
		utils.WriteError(w, http.StatusForbidden, "Your account has been suspended. Please contact the administrator.")
		return
	}

	// Update lastLogin
	now := time.Now()
	_, _ = userColl.UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{"$set": bson.M{"lastLogin": now}})

	token, err := utils.GenerateToken(user.ID.Hex())
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	utils.WriteJSON(w, http.StatusOK, AuthResponse{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
		Role:  user.Role,
		Token: token,
	})
}

// RegisterUser registers a new user
// POST /api/auth/register
func RegisterUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Name == "" || req.Email == "" || req.Password == "" {
		utils.WriteError(w, http.StatusBadRequest, "Name, email, and password are required")
		return
	}

	if req.Role == "master_admin" {
		utils.WriteError(w, http.StatusBadRequest, "Cannot register as master admin")
		return
	}

	userColl := config.DB.Collection("users")
	var existing models.User
	err := userColl.FindOne(r.Context(), bson.M{"email": req.Email}).Decode(&existing)
	if err == nil {
		utils.WriteError(w, http.StatusBadRequest, "User already exists")
		return
	} else if err != mongo.ErrNoDocuments {
		utils.WriteError(w, http.StatusInternalServerError, "Database error")
		return
	}

	hashedPassword, err := models.HashPassword(req.Password)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	role := models.RoleTeamMember
	if req.Role != "" {
		role = models.UserRole(req.Role)
	}

	status := models.StatusActive
	if role == models.RoleTeamHead || role == models.RoleAdmin {
		status = models.StatusPending
	}

	now := time.Now()
	newUser := models.User{
		ID:              primitive.NewObjectID(),
		Name:            req.Name,
		Email:           req.Email,
		Password:        hashedPassword,
		Role:            role,
		Status:          status,
		TeamMembers:     []primitive.ObjectID{},
		TeamName:        "My Team",
		TeamDescription: "Team managed by you",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	_, err = userColl.InsertOne(r.Context(), newUser)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	if newUser.Status == models.StatusPending {
		utils.WriteJSON(w, http.StatusCreated, map[string]any{
			"pending": true,
			"message": "Registration submitted! Your Team Head account is awaiting Master Admin approval. You will be able to log in once approved.",
		})
		return
	}

	token, err := utils.GenerateToken(newUser.ID.Hex())
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	utils.WriteJSON(w, http.StatusCreated, AuthResponse{
		ID:    newUser.ID,
		Name:  newUser.Name,
		Email: newUser.Email,
		Role:  newUser.Role,
		Token: token,
	})
}

// GetUserProfile returns the authenticated user's profile
// GET /api/auth/profile
func GetUserProfile(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "User not found")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"_id":   user.ID,
		"name":  user.Name,
		"email": user.Email,
		"role":  user.Role,
	})
}

// GetAllUsers returns searchable list of users excluding self
// GET /api/auth/users?search=...
func GetAllUsers(w http.ResponseWriter, r *http.Request) {
	currentUser := middleware.GetUserFromContext(r)
	if currentUser == nil {
		utils.WriteError(w, http.StatusUnauthorized, "User not found")
		return
	}

	search := r.URL.Query().Get("search")
	filter := bson.M{
		"_id": bson.M{"$ne": currentUser.ID},
	}

	if search != "" {
		escaped := regexp.QuoteMeta(search)
		filter["$or"] = bson.A{
			bson.M{"name": bson.M{"$regex": escaped, "$options": "i"}},
			bson.M{"email": bson.M{"$regex": escaped, "$options": "i"}},
		}
	}

	userColl := config.DB.Collection("users")
	cursor, err := userColl.Find(r.Context(), filter)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to fetch users")
		return
	}
	defer cursor.Close(r.Context())

	var users []models.User
	if err := cursor.All(r.Context(), &users); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to parse users")
		return
	}

	// Sanitize passwords
	for i := range users {
		users[i].Password = ""
	}

	utils.WriteJSON(w, http.StatusOK, users)
}
