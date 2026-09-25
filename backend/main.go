package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"backend/config"
	"backend/middleware"
	"backend/models"
	"backend/routes"
	"backend/socket"
	"backend/utils"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	// Load .env
	_ = godotenv.Load()

	// Connect to MongoDB
	if _, err := config.ConnectDB(); err != nil {
		log.Fatalf("[Server] Database connection failed: %v", err)
	}

	// Ensure master admin account exists from environment variables
	ensureMasterAdmin()

	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("FATAL ERROR: JWT_SECRET is not defined in environment variables")
	}

	mux := http.NewServeMux()

	// Initialize Socket.io v4 server
	_, socketHandler := socket.InitSocketServer()
	mux.Handle("/socket.io/", socketHandler)

	// Register all REST API routes
	routes.RegisterRoutes(mux)

	// Health check endpoint
	startTime := time.Now()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		uptime := time.Since(startTime).Seconds()
		utils.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"uptime": uptime,
		})
	})

	// Root endpoint (Go 1.22 exact path match using /{$} to avoid wildcard/prefix conflict)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("API is running..."))
	})

	// Static uploads directory serving
	fileServer := http.StripPrefix("/uploads/", http.FileServer(http.Dir(config.UploadsDir)))
	mux.Handle("/uploads/", fileServer)

	// In production, fallback to frontend dist for SPA routing
	var rootHandler http.Handler = mux
	if os.Getenv("NODE_ENV") == "production" {
		frontendDist := filepath.Join("..", "frontend", "dist")
		rootHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/socket.io") || strings.HasPrefix(r.URL.Path, "/uploads/") || r.URL.Path == "/health" || r.URL.Path == "/" {
				mux.ServeHTTP(w, r)
				return
			}
			filePath := filepath.Join(frontendDist, filepath.Clean(r.URL.Path))
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				http.ServeFile(w, r, filePath)
				return
			}
			http.ServeFile(w, r, filepath.Join(frontendDist, "index.html"))
		})
	}

	// Wrap root with CORS and global rate limiting
	handler := middleware.CORSHandler(rootHandler)

	// Background ticker for event reminders (every hour)
	go startEventReminderCron()

	port := os.Getenv("PORT")
	if port == "" {
		port = "4001"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown channel
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[Server] Go Backend listening on http://localhost:%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Server] Listen error: %v", err)
		}
	}()

	<-stop
	log.Println("[Server] Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("[Server] Server forced shutdown: %v", err)
	}

	if config.MongoClient != nil {
		_ = config.MongoClient.Disconnect(shutdownCtx)
	}

	log.Println("[Server] Server exiting")
}

// startEventReminderCron runs every hour to check events starting within 24 hours
func startEventReminderCron() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		if config.DB == nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		now := time.Now()
		tomorrowStart := now.Add(23 * time.Hour)
		tomorrowEnd := now.Add(25 * time.Hour)

		eventColl := config.DB.Collection("events")
		cursor, err := eventColl.Find(ctx, bson.M{
			"start": bson.M{
				"$gte": tomorrowStart,
				"$lte": tomorrowEnd,
			},
		})
		if err != nil {
			cancel()
			continue
		}

		var events []models.Event
		_ = cursor.All(ctx, &events)
		cursor.Close(ctx)

		teamColl := config.DB.Collection("teams")
		for _, ev := range events {
			if ev.Team != nil {
				var team models.Team
				if err := teamColl.FindOne(ctx, bson.M{"_id": ev.Team}).Decode(&team); err == nil {
					var recipientIDs []string
					recipientIDs = append(recipientIDs, team.Owner.Hex())
					for _, m := range team.Members {
						recipientIDs = append(recipientIDs, m.Hex())
					}
					_, _ = utils.CreateNotifications(ctx, recipientIDs, utils.NotificationInput{
						Title:       "Event Reminder",
						Description: fmt.Sprintf("Reminder: The event \"%s\" starts in 1 day!", ev.Title),
						Type:        "event_reminder",
						Link:        "/calendar",
					})
				}
			} else {
				_, _ = utils.CreateNotifications(ctx, []string{ev.User.Hex()}, utils.NotificationInput{
					Title:       "Event Reminder",
					Description: fmt.Sprintf("Reminder: Your event \"%s\" starts in 1 day!", ev.Title),
					Type:        "event_reminder",
					Link:        "/calendar",
				})
			}
		}
		cancel()
	}
}

// ensureMasterAdmin checks environment variables and provisions or updates the master admin account.
func ensureMasterAdmin() {
	masterEmail := strings.TrimSpace(os.Getenv("MASTER_ADMIN_EMAIL"))
	masterPassword := strings.TrimSpace(os.Getenv("MASTER_ADMIN_PASSWORD"))
	masterName := strings.TrimSpace(os.Getenv("MASTER_ADMIN_NAME"))
	if masterName == "" {
		masterName = "Master Admin"
	}

	if masterEmail == "" || masterPassword == "" {
		log.Println("[Server] MASTER_ADMIN_EMAIL or MASTER_ADMIN_PASSWORD not set in environment. Skipping auto-seed.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userColl := config.DB.Collection("users")
	var existing models.User
	err := userColl.FindOne(ctx, bson.M{"email": masterEmail}).Decode(&existing)
	if err == mongo.ErrNoDocuments {
		hashed, err := models.HashPassword(masterPassword)
		if err != nil {
			log.Printf("[Server] Error hashing master admin password: %v", err)
			return
		}
		now := time.Now()
		newAdmin := models.User{
			ID:        primitive.NewObjectID(),
			Name:      masterName,
			Email:     masterEmail,
			Password:  hashed,
			Role:      models.RoleMasterAdmin,
			Status:    models.StatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if _, err := userColl.InsertOne(ctx, newAdmin); err != nil {
			log.Printf("[Server] Failed to create master admin: %v", err)
		} else {
			log.Printf("[Server] Successfully seeded Master Admin account (%s) from environment variables.", masterEmail)
		}
	} else if err == nil {
		// Existing user found: ensure role is master_admin, status is active, and update password if changed
		updateFields := bson.M{
			"role":      models.RoleMasterAdmin,
			"status":    models.StatusActive,
			"updatedAt": time.Now(),
		}
		if !existing.MatchPassword(masterPassword) {
			if hashed, err := models.HashPassword(masterPassword); err == nil {
				updateFields["password"] = hashed
				log.Printf("[Server] Updated master admin password to match environment variable.")
			}
		}
		_, _ = userColl.UpdateOne(ctx, bson.M{"_id": existing.ID}, bson.M{"$set": updateFields})
		log.Printf("[Server] Master Admin account verified and active (%s).", masterEmail)
	} else {
		log.Printf("[Server] Error checking master admin: %v", err)
	}
}
