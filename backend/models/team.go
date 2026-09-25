package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Team struct {
	ID          primitive.ObjectID   `bson:"_id,omitempty" json:"_id"`
	Name        string               `bson:"name" json:"name"`
	Description string               `bson:"description" json:"description"`
	Owner       primitive.ObjectID   `bson:"owner" json:"owner"`
	Members     []primitive.ObjectID `bson:"members" json:"members"`
	CreatedAt   time.Time            `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt   time.Time            `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`

	// Populated fields for API responses
	OwnerDetails  *User  `bson:"-" json:"ownerDetails,omitempty"`
	MemberDetails []User `bson:"-" json:"memberDetails,omitempty"`
}

type TeamPopulatedResponse struct {
	ID          primitive.ObjectID `json:"_id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Owner       *User              `json:"owner"`
	Members     []*User            `json:"members"`
	CreatedAt   time.Time          `json:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
}
