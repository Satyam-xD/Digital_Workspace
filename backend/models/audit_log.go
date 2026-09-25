package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuditLog struct {
	ID         primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	Admin      primitive.ObjectID  `bson:"admin" json:"admin"`
	Action     string              `bson:"action" json:"action"`
	TargetUser *primitive.ObjectID `bson:"targetUser,omitempty" json:"targetUser,omitempty"`
	TargetTeam *primitive.ObjectID `bson:"targetTeam,omitempty" json:"targetTeam,omitempty"`
	Details    string              `bson:"details,omitempty" json:"details,omitempty"`
	CreatedAt  time.Time           `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt  time.Time           `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`

	// Populated fields
	AdminDetails      *UserSummary `bson:"-" json:"adminDetails,omitempty"`
	TargetUserDetails *UserSummary `bson:"-" json:"targetUserDetails,omitempty"`
	TargetTeamDetails any          `bson:"-" json:"targetTeamDetails,omitempty"`
}
