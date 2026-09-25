package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Document struct {
	ID             primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	User           primitive.ObjectID  `bson:"user" json:"user"`
	UploadedBy     primitive.ObjectID  `bson:"uploadedBy" json:"uploadedBy"`
	Name           string              `bson:"name" json:"name"`
	Type           string              `bson:"type" json:"type"`
	Size           string              `bson:"size" json:"size"`
	URL            string              `bson:"url" json:"url"`
	Folder         *primitive.ObjectID `bson:"folder,omitempty" json:"folder"`
	Team           primitive.ObjectID  `bson:"team" json:"team"`
	IsDownloadable bool                `bson:"isDownloadable" json:"isDownloadable"`
	CloudinaryID   *string             `bson:"cloudinaryId,omitempty" json:"cloudinaryId"`
	CreatedAt      time.Time           `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt      time.Time           `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`

	// Populated fields
	UploadedByDetails *UserSummary `bson:"-" json:"uploadedByDetails,omitempty"`
}
