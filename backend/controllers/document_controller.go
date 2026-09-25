package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
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

// GetDocuments returns documents and folders for a team and optional folder
// GET /api/documents?teamId=...&folderId=...
func GetDocuments(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	teamIDStr := r.URL.Query().Get("teamId")
	if teamIDStr == "" {
		utils.WriteError(w, http.StatusBadRequest, "Team ID is required")
		return
	}

	teamObjID, err := primitive.ObjectIDFromHex(teamIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid team ID format")
		return
	}

	folderIDStr := r.URL.Query().Get("folderId")
	var folderFilter any = nil
	if folderIDStr != "" && folderIDStr != "null" && folderIDStr != "root" {
		fID, err := primitive.ObjectIDFromHex(folderIDStr)
		if err == nil {
			folderFilter = fID
		}
	}

	docColl := config.DB.Collection("documents")
	folderColl := config.DB.Collection("folders")

	docFilter := bson.M{
		"team":   teamObjID,
		"folder": folderFilter,
	}

	docCursor, err := docColl.Find(r.Context(), docFilter, options.Find().SetSort(bson.M{"createdAt": -1}))
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load documents")
		return
	}
	defer docCursor.Close(r.Context())

	var documents []models.Document
	_ = docCursor.All(r.Context(), &documents)

	// Fetch folders
	folderFilterQuery := bson.M{
		"team":         teamObjID,
		"parentFolder": folderFilter,
	}
	folderCursor, err := folderColl.Find(r.Context(), folderFilterQuery, options.Find().SetSort(bson.M{"name": 1}))
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to load folders")
		return
	}
	defer folderCursor.Close(r.Context())

	var folders []models.Folder
	_ = folderCursor.All(r.Context(), &folders)

	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"documents": documents,
		"folders":   folders,
	})
}

// UploadDocument handles file upload (PDFs, images, docs) with full inline viewing support
// POST /api/documents/upload
func UploadDocument(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	// 50 MB max
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		utils.WriteError(w, http.StatusBadRequest, "File size too large or invalid form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "No file uploaded")
		return
	}
	_ = file.Close()

	teamIDStr := r.FormValue("teamId")
	if teamIDStr == "" {
		utils.WriteError(w, http.StatusBadRequest, "Team ID is required")
		return
	}

	teamObjID, err := primitive.ObjectIDFromHex(teamIDStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid team ID format")
		return
	}

	folderIDStr := r.FormValue("folderId")
	var folderObjID *primitive.ObjectID
	if folderIDStr != "" && folderIDStr != "null" && folderIDStr != "root" {
		if fID, err := primitive.ObjectIDFromHex(folderIDStr); err == nil {
			folderObjID = &fID
		}
	}

	// Save to local storage engine
	fileURL, fileSize, err := config.SaveUploadedFile(header)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save file: %v", err))
		return
	}

	sizeFormatted := formatFileSize(fileSize)
	ext := strings.TrimPrefix(filepath.Ext(header.Filename), ".")
	if ext == "" {
		ext = "file"
	}

	now := time.Now()
	newDoc := models.Document{
		ID:             primitive.NewObjectID(),
		User:           user.ID,
		UploadedBy:     user.ID,
		Name:           header.Filename,
		Type:           ext,
		Size:           sizeFormatted,
		URL:            fileURL,
		Folder:         folderObjID,
		Team:           teamObjID,
		IsDownloadable: true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	docColl := config.DB.Collection("documents")
	_, err = docColl.InsertOne(r.Context(), newDoc)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to save document record")
		return
	}

	utils.EmitTeamUpdate(teamObjID.Hex(), "DOCUMENT_UPLOAD", nil)

	utils.WriteJSON(w, http.StatusCreated, newDoc)
}

// ViewDocument serves a document (especially PDF) inline in the browser for instant preview
// GET /api/documents/:id/view
func ViewDocument(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	docObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid document ID")
		return
	}

	docColl := config.DB.Collection("documents")
	var doc models.Document
	if err := docColl.FindOne(r.Context(), bson.M{"_id": docObjID}).Decode(&doc); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Document not found")
		return
	}

	// Check if local file
	if strings.HasPrefix(doc.URL, "/uploads/") {
		relPath := strings.TrimPrefix(doc.URL, "/uploads/")
		fullPath := filepath.Join(config.UploadsDir, filepath.Clean(relPath))
		config.ServeFileInline(w, r, fullPath, doc.Name)
		return
	}

	// If remote URL, redirect
	http.Redirect(w, r, doc.URL, http.StatusTemporaryRedirect)
}

// DownloadDocument serves a document as an attachment
// GET /api/documents/:id/download
func DownloadDocument(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	docObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid document ID")
		return
	}

	docColl := config.DB.Collection("documents")
	var doc models.Document
	if err := docColl.FindOne(r.Context(), bson.M{"_id": docObjID}).Decode(&doc); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Document not found")
		return
	}

	if !doc.IsDownloadable {
		utils.WriteError(w, http.StatusForbidden, "Downloads restricted by administrator")
		return
	}

	if strings.HasPrefix(doc.URL, "/uploads/") {
		relPath := strings.TrimPrefix(doc.URL, "/uploads/")
		fullPath := filepath.Join(config.UploadsDir, filepath.Clean(relPath))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, doc.Name))
		http.ServeFile(w, r, fullPath)
		return
	}

	http.Redirect(w, r, doc.URL, http.StatusTemporaryRedirect)
}

// CreateFolder creates a new folder in a team
// POST /api/documents/folder
func CreateFolder(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	var req struct {
		Name           string `json:"name"`
		TeamID         string `json:"teamId"`
		ParentFolder   string `json:"parentFolder"`
		ParentFolderID string `json:"parentFolderId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.TeamID == "" {
		utils.WriteError(w, http.StatusBadRequest, "Folder name and team ID are required")
		return
	}

	teamObjID, err := primitive.ObjectIDFromHex(req.TeamID)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid team ID")
		return
	}

	parentStr := req.ParentFolder
	if parentStr == "" {
		parentStr = req.ParentFolderID
	}

	var parentObjID *primitive.ObjectID
	if parentStr != "" && parentStr != "null" && parentStr != "root" {
		if pID, err := primitive.ObjectIDFromHex(parentStr); err == nil {
			parentObjID = &pID
		}
	}

	now := time.Now()
	newFolder := models.Folder{
		ID:           primitive.NewObjectID(),
		Name:         req.Name,
		Team:         teamObjID,
		CreatedBy:    user.ID,
		ParentFolder: parentObjID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	folderColl := config.DB.Collection("folders")
	_, err = folderColl.InsertOne(r.Context(), newFolder)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to create folder")
		return
	}

	utils.EmitTeamUpdate(teamObjID.Hex(), "FOLDER_CREATE", nil)

	utils.WriteJSON(w, http.StatusCreated, newFolder)
}

// RenameFolder renames an existing folder
// PUT /api/documents/folder/:id
func RenameFolder(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	idStr := r.PathValue("id")
	folderObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid folder ID")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		utils.WriteError(w, http.StatusBadRequest, "New folder name is required")
		return
	}

	folderColl := config.DB.Collection("folders")
	var folder models.Folder
	if err := folderColl.FindOne(r.Context(), bson.M{"_id": folderObjID}).Decode(&folder); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Folder not found")
		return
	}

	isHead := user.Role == models.RoleMasterAdmin || user.Role == models.RoleTeamHead || user.Role == models.RoleAdmin || string(user.Role) == "master"
	if !isHead {
		utils.WriteError(w, http.StatusForbidden, "Only Team Heads can rename folders")
		return
	}

	folder.Name = req.Name
	folder.UpdatedAt = time.Now()
	_, _ = folderColl.UpdateOne(r.Context(), bson.M{"_id": folderObjID}, bson.M{
		"$set": bson.M{"name": folder.Name, "updatedAt": folder.UpdatedAt},
	})

	utils.EmitTeamUpdate(folder.Team.Hex(), "FOLDER_RENAME", nil)
	utils.WriteJSON(w, http.StatusOK, folder)
}

// DeleteFolder recursively deletes a folder and its documents
// DELETE /api/documents/folder/:id
func DeleteFolder(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	if user == nil {
		utils.WriteError(w, http.StatusUnauthorized, "Not authorized")
		return
	}

	idStr := r.PathValue("id")
	folderObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid folder ID")
		return
	}

	folderColl := config.DB.Collection("folders")
	docColl := config.DB.Collection("documents")

	var folder models.Folder
	if err := folderColl.FindOne(r.Context(), bson.M{"_id": folderObjID}).Decode(&folder); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Folder not found")
		return
	}

	isHead := user.Role == models.RoleMasterAdmin || user.Role == models.RoleTeamHead || user.Role == models.RoleAdmin || string(user.Role) == "master"
	if !isHead {
		utils.WriteError(w, http.StatusForbidden, "Only Team Heads can delete folders")
		return
	}

	// Recursive deletion helper
	var deleteRecursive func(fID primitive.ObjectID)
	deleteRecursive = func(fID primitive.ObjectID) {
		cursor, err := folderColl.Find(r.Context(), bson.M{"parentFolder": fID})
		if err == nil {
			var subs []models.Folder
			_ = cursor.All(r.Context(), &subs)
			cursor.Close(r.Context())
			for _, sub := range subs {
				deleteRecursive(sub.ID)
			}
		}

		docCursor, err := docColl.Find(r.Context(), bson.M{"folder": fID})
		if err == nil {
			var docs []models.Document
			_ = docCursor.All(r.Context(), &docs)
			docCursor.Close(r.Context())
			for _, d := range docs {
				_ = config.DeleteFile(d.URL)
				_, _ = docColl.DeleteOne(r.Context(), bson.M{"_id": d.ID})
			}
		}
		_, _ = folderColl.DeleteOne(r.Context(), bson.M{"_id": fID})
	}

	deleteRecursive(folderObjID)

	utils.EmitTeamUpdate(folder.Team.Hex(), "FOLDER_DELETE", nil)
	utils.WriteJSON(w, http.StatusOK, map[string]any{"id": idStr, "message": "Folder deleted"})
}


// DeleteDocument deletes a document
// DELETE /api/documents/:id
func DeleteDocument(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	docObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid document ID")
		return
	}

	docColl := config.DB.Collection("documents")
	var doc models.Document
	if err := docColl.FindOne(r.Context(), bson.M{"_id": docObjID}).Decode(&doc); err != nil {
		utils.WriteError(w, http.StatusNotFound, "Document not found")
		return
	}

	// Delete from storage
	_ = config.DeleteFile(doc.URL)

	_, _ = docColl.DeleteOne(r.Context(), bson.M{"_id": docObjID})
	utils.EmitTeamUpdate(doc.Team.Hex(), "DOCUMENT_DELETE", nil)

	utils.WriteJSON(w, http.StatusOK, map[string]any{"id": idStr})
}

// ToggleDownloadable toggles download permission
// PATCH /api/documents/:id/downloadable
func ToggleDownloadable(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	docObjID, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "Invalid document ID")
		return
	}

	var req struct {
		IsDownloadable bool `json:"isDownloadable"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	docColl := config.DB.Collection("documents")
	_, err = docColl.UpdateOne(r.Context(), bson.M{"_id": docObjID}, bson.M{"$set": bson.M{"isDownloadable": req.IsDownloadable}})
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "Failed to update download permissions")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]any{"isDownloadable": req.IsDownloadable})
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Helper to parse int with fallback
func parseIntDefault(s string, def int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
