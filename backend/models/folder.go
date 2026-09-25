package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Folder struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty" json:"_id"`
	Name         string              `bson:"name" json:"name"`
	Team         primitive.ObjectID  `bson:"team" json:"team"`
	CreatedBy    primitive.ObjectID  `bson:"createdBy" json:"createdBy"`
	ParentFolder *primitive.ObjectID `bson:"parentFolder,omitempty" json:"parentFolder"`
	CreatedAt    time.Time           `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt    time.Time           `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`

	// Populated fields
	CreatedByDetails *UserSummary `bson:"-" json:"createdByDetails,omitempty"`
}
