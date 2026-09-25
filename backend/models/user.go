package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

type UserRole string

const (
	RoleTeamMember  UserRole = "team_member"
	RoleTeamHead    UserRole = "team_head"
	RoleAdmin       UserRole = "admin"
	RoleMasterAdmin UserRole = "master_admin"
)

type UserStatus string

const (
	StatusPending   UserStatus = "pending"
	StatusActive    UserStatus = "active"
	StatusSuspended UserStatus = "suspended"
)

type User struct {
	ID              primitive.ObjectID   `bson:"_id,omitempty" json:"_id"`
	Name            string               `bson:"name" json:"name"`
	Email           string               `bson:"email" json:"email"`
	Password        string               `bson:"password,omitempty" json:"password,omitempty"`
	Role            UserRole             `bson:"role" json:"role"`
	Status          UserStatus           `bson:"status" json:"status"`
	TeamMembers     []primitive.ObjectID `bson:"teamMembers,omitempty" json:"teamMembers,omitempty"`
	TeamName        string               `bson:"teamName,omitempty" json:"teamName,omitempty"`
	TeamDescription string               `bson:"teamDescription,omitempty" json:"teamDescription,omitempty"`
	LastLogin       *time.Time           `bson:"lastLogin,omitempty" json:"lastLogin,omitempty"`
	CreatedAt       time.Time            `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt       time.Time            `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`

	// Virtual fields for responses
	TasksAssigned  int      `bson:"-" json:"tasksAssigned,omitempty"`
	TasksCompleted int      `bson:"-" json:"tasksCompleted,omitempty"`
	Teams          []string `bson:"-" json:"teams,omitempty"`
	IsOwner        bool     `bson:"-" json:"isOwner,omitempty"`
}

// HashPassword hashes a raw password using bcrypt.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(bytes), err
}

// MatchPassword compares a plaintext password with its bcrypt hash.
func (u *User) MatchPassword(plainPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(plainPassword))
	return err == nil
}
