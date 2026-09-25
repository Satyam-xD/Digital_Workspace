package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Password struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"_id"`
	User      primitive.ObjectID `bson:"user" json:"user"`
	Title     string             `bson:"title" json:"title"`
	Username  string             `bson:"username" json:"username"`
	Password  string             `bson:"password" json:"password"`
	URL       string             `bson:"url,omitempty" json:"url,omitempty"`
	Category  string             `bson:"category" json:"category"` // 'login', 'meeting', 'website', 'other'
	Notes     string             `bson:"notes,omitempty" json:"notes,omitempty"`
	CreatedAt time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}
