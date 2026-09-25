package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
)

var (
	DB          *mongo.Database
	MongoClient *mongo.Client
)

// ConnectDB establishes a connection to MongoDB using the MONGO_URI environment variable.
func ConnectDB() (*mongo.Database, error) {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("Error: MONGO_URI is not defined in environment")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("MongoDB ping failed: %w", err)
	}

	// Parse database name from URI, default to "aurora" if not specified
	dbName := "aurora"
	cs, err := connstring.ParseAndValidate(mongoURI)
	if err == nil && cs.Database != "" {
		dbName = cs.Database
	}

	MongoClient = client
	DB = client.Database(dbName)

	log.Printf("[Database] MongoDB Connected successfully to database: %s", dbName)
	return DB, nil
}
