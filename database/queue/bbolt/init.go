package bbolt

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-bolt/pkg/bolt"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"go.etcd.io/bbolt"
)

// BoltQueueService implements QueueEngineInterface using Watermill Bolt Pub/Sub
type BoltQueueService struct {
	publisher  message.Publisher
	subscriber message.Subscriber
	logger     watermill.LoggerAdapter
	dbPath     string
}

// Ensure BoltQueueService implements QueueEngineInterface
var _ interfaces.QueueEngineInterface = (*BoltQueueService)(nil)

func GetBoltQueueDriver(cfg *models.Config) (*BoltQueueService, error) {
	logger := watermill.NewStdLogger(false, false)

	// Use system database path as base for queue database
	var dbPath string
	if cfg.QueueStorageEngineDatabase != "" {
		dbPath = cfg.QueueStorageEngineDatabase
	} else {
		// Expand home directory and create default path
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		dbPath = filepath.Join(homeDir, ".apito", "engine-data", "apito_queue.db")
	}

	// Expand path (handles ~ and converts to absolute path)
	var err error
	dbPath, err = utility.ExpandPath(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand database path %s: %v", dbPath, err)
	}

	// Create directory if it doesn't exist
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
	}

	// Open BoltDB database
	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt database: %w", err)
	}

	// Create Bolt Publisher
	publisher, err := bolt.NewPublisher(db, bolt.PublisherConfig{
		Common: bolt.CommonConfig{
			Logger: logger,
			Bucket: []bolt.BucketName{bolt.BucketName("messages")},
		},
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create bolt publisher: %w", err)
	}

	// Create Bolt Subscriber
	subscriber, err := bolt.NewSubscriber(db, bolt.SubscriberConfig{
		Common: bolt.CommonConfig{
			Logger: logger,
			Bucket: []bolt.BucketName{bolt.BucketName("messages")},
		},
	})
	if err != nil {
		publisher.Close()
		db.Close()
		return nil, fmt.Errorf("failed to create bolt subscriber: %w", err)
	}

	return &BoltQueueService{
		publisher:  publisher,
		subscriber: subscriber,
		logger:     logger,
		dbPath:     dbPath,
	}, nil
}
