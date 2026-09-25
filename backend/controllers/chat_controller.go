package controllers

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backend/config"
	"backend/middleware"
	"backend/models"
	"backend/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetChatHistory fetches paginated chat history for a room
// GET /api/chat/history/:room
func GetChatHistory(w http.ResponseWriter, r *http.Request) {
	room := r.PathValue("room")
	roomObjID, err := primitive.ObjectIDFromHex(room)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid room ID format")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	skip := int64((page - 1) * limit)

	msgColl := config.DB.Collection("messages")
	userColl := config.DB.Collection("users")

	totalMessages, _ := msgColl.CountDocuments(r.Context(), bson.M{"chat": roomObjID})

	opts := options.Find().
		SetSort(bson.M{"createdAt": -1}).
		SetSkip(skip).
		SetLimit(int64(limit))

	cursor, err := msgColl.Find(r.Context(), bson.M{"chat": roomObjID}, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load messages")
		return
	}
	defer cursor.Close(r.Context())

	var messages []models.Message
	_ = cursor.All(r.Context(), &messages)

	// Populate sender info
	for i := range messages {
		var sender models.UserSummary
		if err := userColl.FindOne(r.Context(), bson.M{"_id": messages[i].Sender}).Decode(&sender); err == nil {
			messages[i].SenderDetails = sender
		}
	}

	// Reverse to oldest first for UI
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"messages": messages,
		"pagination": map[string]any{
			"currentPage":   page,
			"totalPages":    int(math.Ceil(float64(totalMessages) / float64(limit))),
			"totalMessages": totalMessages,
			"hasMore":       skip+int64(len(messages)) < totalMessages,
		},
	})
}

// SendMessage creates a new chat message
// POST /api/chat/message
func SendMessage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req struct {
		ChatID string `json:"chatId"`
		Text   string `json:"text"`
		Type   string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChatID == "" || req.Text == "" {
		utils.WriteError(w, http.StatusBadRequest, "ChatId and text are required")
		return
	}

	chatObjID, err := primitive.ObjectIDFromHex(req.ChatID)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid chat ID format")
		return
	}

	msgType := "text"
	if req.Type != "" {
		msgType = req.Type
	}

	now := time.Now()
	newMsg := models.Message{
		ID:        primitive.NewObjectID(),
		Chat:      chatObjID,
		Sender:    user.ID,
		Text:      req.Text,
		Type:      msgType,
		CreatedAt: now,
		UpdatedAt: now,
	}

	msgColl := config.DB.Collection("messages")
	_, err = msgColl.InsertOne(r.Context(), newMsg)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to send message")
		return
	}

	// Update latest message in conversation
	convColl := config.DB.Collection("conversations")
	_, _ = convColl.UpdateOne(r.Context(), bson.M{"_id": chatObjID}, bson.M{"$set": bson.M{"latestMessage": newMsg.ID, "updatedAt": now}})

	newMsg.SenderDetails = models.UserSummary{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
	}

	utils.WriteJSON(w, http.StatusCreated, newMsg)
}

// AccessChat accesses or creates a 1-on-1 chat
// POST /api/chat
func AccessChat(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req struct {
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		utils.WriteError(w, http.StatusBadRequest, "UserId param not sent with request")
		return
	}

	targetObjID, err := primitive.ObjectIDFromHex(req.UserID)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	convColl := config.DB.Collection("conversations")
	filter := bson.M{
		"isGroupChat": false,
		"$and": bson.A{
			bson.M{"users": bson.M{"$elemMatch": bson.M{"$eq": user.ID}}},
			bson.M{"users": bson.M{"$elemMatch": bson.M{"$eq": targetObjID}}},
		},
	}

	var existing models.Conversation
	err = convColl.FindOne(r.Context(), filter).Decode(&existing)
	if err == nil {
		populateConversation(r, &existing)
		utils.WriteJSON(w, http.StatusOK, existing)
		return
	}

	// Create new chat
	now := time.Now()
	newChat := models.Conversation{
		ID:          primitive.NewObjectID(),
		ChatName:    "sender",
		IsGroupChat: false,
		Users:       []primitive.ObjectID{user.ID, targetObjID},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err = convColl.InsertOne(r.Context(), newChat)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create conversation")
		return
	}

	populateConversation(r, &newChat)
	utils.WriteJSON(w, http.StatusOK, newChat)
}

// FetchChats returns all conversations for the user
// GET /api/chat
func FetchChats(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	convColl := config.DB.Collection("conversations")
	opts := options.Find().SetSort(bson.M{"updatedAt": -1})
	cursor, err := convColl.Find(r.Context(), bson.M{"users": bson.M{"$elemMatch": bson.M{"$eq": user.ID}}}, opts)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load chats")
		return
	}
	defer cursor.Close(r.Context())

	var chats []models.Conversation
	_ = cursor.All(r.Context(), &chats)

	for i := range chats {
		populateConversation(r, &chats[i])
	}

	utils.WriteJSON(w, http.StatusOK, chats)
}

// CreateGroupChat creates a group conversation
// POST /api/chat/group
func CreateGroupChat(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req struct {
		Name  string `json:"name"`
		Users string `json:"users"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		utils.WriteError(w, http.StatusBadRequest, "Group name is required")
		return
	}

	var userIDs []string
	if req.Users != "" {
		_ = json.Unmarshal([]byte(req.Users), &userIDs)
	}

	userObjSet := make(map[string]primitive.ObjectID)
	userObjSet[user.ID.Hex()] = user.ID
	for _, idStr := range userIDs {
		if oID, err := primitive.ObjectIDFromHex(idStr); err == nil {
			userObjSet[oID.Hex()] = oID
		}
	}

	var memberList []primitive.ObjectID
	for _, id := range userObjSet {
		memberList = append(memberList, id)
	}

	now := time.Now()
	group := models.Conversation{
		ID:          primitive.NewObjectID(),
		ChatName:    req.Name,
		IsGroupChat: true,
		Users:       memberList,
		GroupAdmin:  &user.ID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	convColl := config.DB.Collection("conversations")
	_, err := convColl.InsertOne(r.Context(), group)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create group")
		return
	}

	populateConversation(r, &group)
	utils.WriteJSON(w, http.StatusOK, group)
}

// UploadChatAttachment handles file/image upload for chat
// POST /api/chat/upload
func UploadChatAttachment(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid upload form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "No file selected")
		return
	}
	_ = file.Close()

	fileURL, _, err := config.SaveUploadedFile(header)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to save file")
		return
	}

	contentType := header.Header.Get("Content-Type")
	fileType := "file"
	if strings.HasPrefix(contentType, "image/") {
		fileType = "image"
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{
		"url":  fileURL,
		"type": fileType,
	})
}

// RenameGroup updates the name of a group chat
// PUT /api/chat/rename
func RenameGroup(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req struct {
		ChatID   string `json:"chatId"`
		ChatName string `json:"chatName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChatID == "" || req.ChatName == "" {
		utils.WriteError(w, http.StatusBadRequest, "chatId and chatName are required")
		return
	}

	cID, err := primitive.ObjectIDFromHex(req.ChatID)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid chat ID")
		return
	}

	convColl := config.DB.Collection("conversations")
	_, err = convColl.UpdateOne(r.Context(), bson.M{"_id": cID}, bson.M{"$set": bson.M{"chatName": req.ChatName, "updatedAt": time.Now()}})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to update chat name")
		return
	}

	var updated models.Conversation
	if err := convColl.FindOne(r.Context(), bson.M{"_id": cID}).Decode(&updated); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Chat not found")
		return
	}

	populateConversation(r, &updated)
	utils.WriteJSON(w, http.StatusOK, updated)
}

// AddToGroup adds a user to a group chat
// PUT /api/chat/groupadd
func AddToGroup(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req struct {
		ChatID string `json:"chatId"`
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChatID == "" || req.UserID == "" {
		utils.WriteError(w, http.StatusBadRequest, "chatId and userId are required")
		return
	}

	cID, err := primitive.ObjectIDFromHex(req.ChatID)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid chat ID")
		return
	}
	uID, err := primitive.ObjectIDFromHex(req.UserID)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	convColl := config.DB.Collection("conversations")
	_, err = convColl.UpdateOne(r.Context(), bson.M{"_id": cID}, bson.M{
		"$addToSet": bson.M{"users": uID},
		"$set":      bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to add user to group")
		return
	}

	var updated models.Conversation
	if err := convColl.FindOne(r.Context(), bson.M{"_id": cID}).Decode(&updated); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Chat not found")
		return
	}

	populateConversation(r, &updated)
	utils.WriteJSON(w, http.StatusOK, updated)
}

// RemoveFromGroup removes a user from a group chat
// PUT /api/chat/groupremove
func RemoveFromGroup(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req struct {
		ChatID string `json:"chatId"`
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChatID == "" || req.UserID == "" {
		utils.WriteError(w, http.StatusBadRequest, "chatId and userId are required")
		return
	}

	cID, err := primitive.ObjectIDFromHex(req.ChatID)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid chat ID")
		return
	}
	uID, err := primitive.ObjectIDFromHex(req.UserID)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	convColl := config.DB.Collection("conversations")
	_, err = convColl.UpdateOne(r.Context(), bson.M{"_id": cID}, bson.M{
		"$pull": bson.M{"users": uID},
		"$set":  bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to remove user from group")
		return
	}

	var updated models.Conversation
	if err := convColl.FindOne(r.Context(), bson.M{"_id": cID}).Decode(&updated); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Chat not found")
		return
	}

	populateConversation(r, &updated)
	utils.WriteJSON(w, http.StatusOK, updated)
}


func populateConversation(r *http.Request, conv *models.Conversation) {
	userColl := config.DB.Collection("users")
	msgColl := config.DB.Collection("messages")

	for _, uID := range conv.Users {
		var u models.UserSummary
		if err := userColl.FindOne(r.Context(), bson.M{"_id": uID}).Decode(&u); err == nil {
			conv.UsersDetails = append(conv.UsersDetails, &u)
		}
	}

	if conv.GroupAdmin != nil {
		var a models.UserSummary
		if err := userColl.FindOne(r.Context(), bson.M{"_id": conv.GroupAdmin}).Decode(&a); err == nil {
			conv.GroupAdminDetails = &a
		}
	}

	if conv.LatestMessage != nil {
		var m models.Message
		if err := msgColl.FindOne(r.Context(), bson.M{"_id": conv.LatestMessage}).Decode(&m); err == nil {
			var s models.UserSummary
			_ = userColl.FindOne(r.Context(), bson.M{"_id": m.Sender}).Decode(&s)
			m.SenderDetails = s
			conv.LatestMessageDetails = m
		}
	}
}
