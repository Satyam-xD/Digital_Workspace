package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Activity struct {
	ID        primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	TeamOwner primitive.ObjectID  `bson:"teamOwner" json:"teamOwner"`
	Team      *primitive.ObjectID `bson:"team,omitempty" json:"team,omitempty"`
	Text      string              `bson:"text" json:"text"`
	Type      string              `bson:"type" json:"type"`
	CreatedAt time.Time           `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt time.Time           `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}
