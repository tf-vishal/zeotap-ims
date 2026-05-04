package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tf-vishal/zeotap-ims/internal/config"
	"github.com/tf-vishal/zeotap-ims/internal/models"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MongoClient wraps the MongoDB connection and provides domain-specific methods.
type MongoClient struct {
	client *mongo.Client
	db     *mongo.Database
}

// NewMongoClient connects to MongoDB and verifies the connection.
func NewMongoClient(cfg *config.Config) (*MongoClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOpts := options.Client().ApplyURI(cfg.MongoURI)
	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	log.Printf("[mongo] connected to %s (db=%s)", cfg.MongoURI, cfg.MongoDB)
	return &MongoClient{
		client: client,
		db:     client.Database(cfg.MongoDB),
	}, nil
}

// InsertSignalAudit appends a raw signal to the signal_audits collection.
func (m *MongoClient) InsertSignalAudit(ctx context.Context, audit *models.SignalAudit) error {
	_, err := m.db.Collection("signal_audits").InsertOne(ctx, audit)
	return err
}

// Ping checks MongoDB connectivity (used by health endpoint).
func (m *MongoClient) Ping(ctx context.Context) error {
	return m.client.Ping(ctx, nil)
}

// Close disconnects from MongoDB.
func (m *MongoClient) Close(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}
