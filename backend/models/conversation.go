package models

import (
	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Conversation struct {
	ID            primitive.ObjectID   `bson:"_id,omitempty" json:"_id"`
	ChatName      string               `bson:"chatName,omitempty" json:"chatName,omitempty"`
	IsGroupChat   bool                 `bson:"isGroupChat" json:"isGroupChat"`
	Users         []primitive.ObjectID `bson:"users" json:"users"`
	LatestMessage *primitive.ObjectID  `bson:"latestMessage,omitempty" json:"latestMessage,omitempty"`
	GroupAdmin    *primitive.ObjectID  `bson:"groupAdmin,omitempty" json:"groupAdmin,omitempty"`
	CreatedAt     time.Time            `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt     time.Time            `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`

	// Populated for responses
	UsersDetails         []*UserSummary `bson:"-" json:"usersDetails,omitempty"`
	GroupAdminDetails    *UserSummary   `bson:"-" json:"groupAdminDetails,omitempty"`
	LatestMessageDetails any            `bson:"-" json:"latestMessageDetails,omitempty"`
}

type conversationJSON struct {
	ID            primitive.ObjectID `json:"_id"`
	ChatName      string             `json:"chatName,omitempty"`
	IsGroupChat   bool               `json:"isGroupChat"`
	Users         []*UserSummary     `json:"users"`
	LatestMessage any                `json:"latestMessage,omitempty"`
	GroupAdmin    *UserSummary       `json:"groupAdmin,omitempty"`
	CreatedAt     time.Time          `json:"createdAt,omitempty"`
	UpdatedAt     time.Time          `json:"updatedAt,omitempty"`
}

// MarshalJSON provides exact Mongoose populated output for frontend compatibility
func (c Conversation) MarshalJSON() ([]byte, error) {
	users := c.UsersDetails
	if users == nil {
		users = []*UserSummary{}
	}
	return json.Marshal(conversationJSON{
		ID:            c.ID,
		ChatName:      c.ChatName,
		IsGroupChat:   c.IsGroupChat,
		Users:         users,
		LatestMessage: c.LatestMessageDetails,
		GroupAdmin:    c.GroupAdminDetails,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	})
}
