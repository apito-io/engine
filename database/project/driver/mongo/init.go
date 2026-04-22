package mongo

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// atlasBootstrapTimeout matches system driver: SRV/DNS/TLS/server selection often exceeds 10s on Atlas.
const atlasBootstrapTimeout = 45 * time.Second

type MongoDriver struct {
	Conf             *models.Config
	Client           *mongo.Client
	Database         *mongo.Database
	DriverCredential *models.DriverCredentials
}

// buildProjectMongoURI builds a MongoDB connection URI (same rules as system driver mongodb package).
// Empty port implies mongodb+srv (Atlas / SRV).
func buildProjectMongoURI(c *models.DriverCredentials) (string, bool, error) {
	host := strings.TrimSpace(c.Host)
	if host == "" {
		return "", false, fmt.Errorf("MongoDB host is required")
	}
	db := strings.Trim(strings.TrimSpace(c.Database), "/")
	port := strings.TrimSpace(c.Port)

	if port == "" {
		u := &url.URL{Scheme: "mongodb+srv", Host: host}
		if strings.TrimSpace(c.User) != "" || strings.TrimSpace(c.Password) != "" {
			u.User = url.UserPassword(c.User, c.Password)
		}
		if db != "" {
			u.Path = "/" + db
		}
		q := url.Values{}
		q.Set("retryWrites", "true")
		q.Set("w", "majority")
		if u.User != nil {
			q.Set("authSource", "admin")
		}
		if db != "" {
			q.Set("appName", db)
		}
		u.RawQuery = q.Encode()
		return u.String(), true, nil
	}

	u := &url.URL{
		Scheme: "mongodb",
		Host:   net.JoinHostPort(host, port),
	}
	if strings.TrimSpace(c.User) != "" || strings.TrimSpace(c.Password) != "" {
		u.User = url.UserPassword(c.User, c.Password)
	}
	if db != "" {
		u.Path = "/" + db
	}
	q := url.Values{}
	q.Set("retryWrites", "true")
	q.Set("w", "majority")
	if u.User != nil {
		q.Set("authSource", "admin")
	}
	u.RawQuery = q.Encode()
	return u.String(), false, nil
}

func GetProjectMongoDriver(conf *models.Config, driverCredentials *models.DriverCredentials) (*MongoDriver, error) {
	connectionURI, useSRV, err := buildProjectMongoURI(driverCredentials)
	if err != nil {
		return nil, err
	}
	bootstrapTimeout := 20 * time.Second
	if useSRV {
		bootstrapTimeout = atlasBootstrapTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), bootstrapTimeout)
	defer cancel()

	opts := options.Client().ApplyURI(connectionURI)
	if useSRV {
		opts.SetServerAPIOptions(options.ServerAPI(options.ServerAPIVersion1))
		opts.SetServerSelectionTimeout(bootstrapTimeout - 2*time.Second)
		opts.SetConnectTimeout(30 * time.Second)
	}
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("MongoDB connection failed: %w", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("MongoDB ping failed (check credentials and network): %w", err)
	}

	database := client.Database(driverCredentials.Database)

	return &MongoDriver{
		Conf:             conf,
		Client:           client,
		Database:         database,
		DriverCredential: driverCredentials,
	}, nil
}

func (m *MongoDriver) Ping() error {
	return m.Client.Ping(context.Background(), nil)
}
