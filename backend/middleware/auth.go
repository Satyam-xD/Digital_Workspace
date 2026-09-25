package middleware

import (
	"context"
	"net/http"
	"strings"

	"backend/config"
	"backend/models"
	"backend/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type contextKey string

const UserContextKey contextKey = "user"

// GetUserFromContext retrieves the authenticated user from the request context
func GetUserFromContext(r *http.Request) *models.User {
	if user, ok := r.Context().Value(UserContextKey).(*models.User); ok {
		return user
	}
	return nil
}

// Protect verifies the JWT Bearer token and checks user status
func Protect(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			utils.WriteError(w, http.StatusUnauthorized, "Not authorized, no token")
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := utils.VerifyToken(tokenString)
		if err != nil {
			utils.WriteError(w, http.StatusUnauthorized, "Not authorized, token failed")
			return
		}

		objID, err := primitive.ObjectIDFromHex(claims.ID)
		if err != nil {
			utils.WriteError(w, http.StatusUnauthorized, "Invalid token ID format")
			return
		}

		var user models.User
		collection := config.DB.Collection("users")
		err = collection.FindOne(r.Context(), bson.M{"_id": objID}).Decode(&user)
		if err != nil {
			utils.WriteError(w, http.StatusUnauthorized, "User not found")
			return
		}

		if user.Status == models.StatusSuspended {
			utils.WriteError(w, http.StatusUnauthorized, "Not authorized, account has been suspended by the master admin")
			return
		}

		if user.Status == models.StatusPending {
			utils.WriteError(w, http.StatusForbidden, "Not authorized, account is pending approval")
			return
		}

		// Don't leak hashed password
		user.Password = ""

		ctx := context.WithValue(r.Context(), UserContextKey, &user)
		next(w, r.WithContext(ctx))
	}
}

// Admin checks if the user is a team_head, admin, master, or master_admin
func Admin(next http.HandlerFunc) http.HandlerFunc {
	return Protect(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r)
		if user == nil {
			utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
			return
		}

		if user.Role == models.RoleTeamHead || user.Role == models.RoleAdmin || user.Role == models.RoleMasterAdmin || string(user.Role) == "master" {
			next(w, r)
			return
		}

		utils.WriteError(w, http.StatusUnauthorized, "Not authorized as an admin")
	})
}

// MasterAdmin checks if the user is explicitly a master_admin
func MasterAdmin(next http.HandlerFunc) http.HandlerFunc {
	return Protect(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r)
		if user == nil || user.Role != models.RoleMasterAdmin {
			utils.WriteError(w, http.StatusUnauthorized, "Not authorized as a master admin")
			return
		}

		next(w, r)
	})
}
