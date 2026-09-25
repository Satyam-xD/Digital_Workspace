package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Event struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	Title       string              `bson:"title" json:"title"`
	Start       time.Time           `bson:"start" json:"start"`
	End         time.Time           `bson:"end" json:"end"`
	AllDay      bool                `bson:"allDay" json:"allDay"`
	User        primitive.ObjectID  `bson:"user" json:"user"`
	Team        *primitive.ObjectID `bson:"team,omitempty" json:"team,omitempty"`
	IsGlobal    bool                `bson:"isGlobal" json:"isGlobal"`
	Description string              `bson:"description,omitempty" json:"description,omitempty"`
	Color       string              `bson:"color" json:"color"`
	CreatedAt   time.Time           `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt   time.Time           `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}
