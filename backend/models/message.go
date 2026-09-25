package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	Chat      primitive.ObjectID `bson:"chat" json:"chat"`
	Sender    primitive.ObjectID `bson:"sender" json:"sender"`
	Text      string             `bson:"text" json:"text"`
	Type      string             `bson:"type" json:"type"` // 'text', 'image', 'file'
	CreatedAt time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`

	// Populated fields
	SenderDetails any `bson:"-" json:"senderDetails,omitempty"`
	ChatDetails   any `bson:"-" json:"chatDetails,omitempty"`
}

type MessagePopulated struct {
	ID        primitive.ObjectID `json:"_id"`
	Chat      any                `json:"chat"`
	Sender    *UserSummary       `json:"sender"`
	Text      string             `json:"text"`
	Type      string             `json:"type"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
}
