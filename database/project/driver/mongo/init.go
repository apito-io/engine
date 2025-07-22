package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoDriver struct {
	Client           *mongo.Client
	Database         *mongo.Database
	DriverCredential *models.DriverCredentials
}

func GetMongoDriver(driverCredentials *models.DriverCredentials) (*MongoDriver, error) {
	// Create MongoDB connection string
	connectionURI := fmt.Sprintf("mongodb+srv://%s:%s@%s/%s?retryWrites=true&w=majority",
		driverCredentials.User,
		driverCredentials.Password,
		driverCredentials.Host,
		//driverCredentials.Port,
		driverCredentials.Database)

	// Set connection timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create client options and connect to MongoDB
	opts := options.Client().ApplyURI(connectionURI)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, err
	}

	// Ping the database to verify connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Get database
	database := client.Database(driverCredentials.Database)

	return &MongoDriver{
		Client:           client,
		Database:         database,
		DriverCredential: driverCredentials,
	}, nil
}
