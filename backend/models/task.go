package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Task struct {
	ID             primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	User           primitive.ObjectID  `bson:"user" json:"user"`
	AssignedTo     *primitive.ObjectID `bson:"assignedTo,omitempty" json:"assignedTo,omitempty"`
	Team           *primitive.ObjectID `bson:"team,omitempty" json:"team,omitempty"`
	Title          string              `bson:"title" json:"title"`
	Description    string              `bson:"description,omitempty" json:"description,omitempty"`
	Status         string              `bson:"status" json:"status"`
	Priority       string              `bson:"priority" json:"priority"`
	Tag            string              `bson:"tag" json:"tag"`
	DueDate        *time.Time          `bson:"dueDate,omitempty" json:"dueDate,omitempty"`
	CompletedBy    *primitive.ObjectID `bson:"completedBy,omitempty" json:"completedBy,omitempty"`
	CompletedAt    *time.Time          `bson:"completedAt,omitempty" json:"completedAt,omitempty"`
	LastModifiedBy *primitive.ObjectID `bson:"lastModifiedBy,omitempty" json:"lastModifiedBy,omitempty"`
	CreatedAt      time.Time           `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt      time.Time           `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`

	// Populated fields for client
	AssignedToDetails     any `bson:"-" json:"assignedToDetails,omitempty"`
	CompletedByDetails    any `bson:"-" json:"completedByDetails,omitempty"`
	LastModifiedByDetails any `bson:"-" json:"lastModifiedByDetails,omitempty"`
}

type UserSummary struct {
	ID    primitive.ObjectID `json:"_id" bson:"_id"`
	Name  string             `json:"name" bson:"name"`
	Email string             `json:"email" bson:"email"`
}
