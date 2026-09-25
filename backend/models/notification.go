package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Notification struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	Recipient   primitive.ObjectID  `bson:"recipient" json:"recipient"`
	Sender      *primitive.ObjectID `bson:"sender,omitempty" json:"sender,omitempty"`
	Title       string              `bson:"title" json:"title"`
	Description string              `bson:"description" json:"description"`
	Type        string              `bson:"type" json:"type"`
	Link        string              `bson:"link,omitempty" json:"link,omitempty"`
	Read        bool                `bson:"read" json:"read"`
	CreatedAt   time.Time           `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt   time.Time           `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`

	// Populated fields
	SenderDetails *UserSummary `bson:"-" json:"senderDetails,omitempty"`
}
