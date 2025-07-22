package couchbase

import (
	"context"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/couchbase/gocb/v2"
)

type CouchbaseDriver struct {
	Cluster          *gocb.Cluster
	Bucket           *gocb.Bucket
	Collection       *gocb.Collection
	DriverCredential *models.DriverCredentials
}

// GetCouchbaseDriver creates a new Couchbase project driver instance
func GetCouchbaseDriver(driverCredentials *models.DriverCredentials) (*CouchbaseDriver, error) {
	// Connect to Couchbase cluster
	cluster, err := gocb.Connect(
		fmt.Sprintf("couchbase://%s", driverCredentials.Host),
		gocb.ClusterOptions{
			Authenticator: gocb.PasswordAuthenticator{
				Username: driverCredentials.User,
				Password: driverCredentials.Password,
			},
		})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Couchbase: %v", err)
	}

	// Wait for connection to be ready
	err = cluster.WaitUntilReady(10*time.Second, nil)
	if err != nil {
		return nil, fmt.Errorf("cluster not ready: %v", err)
	}

	// Open bucket
	bucket := cluster.Bucket(driverCredentials.Database)
	err = bucket.WaitUntilReady(5*time.Second, nil)
	if err != nil {
		return nil, fmt.Errorf("bucket not ready: %v", err)
	}

	// Get default collection
	collection := bucket.DefaultCollection()

	return &CouchbaseDriver{
		Cluster:          cluster,
		Bucket:           bucket,
		Collection:       collection,
		DriverCredential: driverCredentials,
	}, nil
}

// Close closes the Couchbase connection
func (c *CouchbaseDriver) Close() {
	if c.Cluster != nil {
		c.Cluster.Close(nil)
	}
}

// DeleteProject deletes a project and all related data
func (c *CouchbaseDriver) DeleteProject(ctx context.Context, projectID string) error {
	// Delete all project-related documents using N1QL
	queries := []string{
		fmt.Sprintf("DELETE FROM `%s` WHERE doc_type = \"project_document\" AND project_id = \"%s\"", c.Bucket.Name(), projectID),
		fmt.Sprintf("DELETE FROM `%s` WHERE doc_type = \"project_relation\" AND project_id = \"%s\"", c.Bucket.Name(), projectID),
		fmt.Sprintf("DELETE FROM `%s` WHERE doc_type = \"project_revision\" AND project_id = \"%s\"", c.Bucket.Name(), projectID),
		fmt.Sprintf("DELETE FROM `%s` WHERE doc_type = \"project_builder\" AND project_id = \"%s\"", c.Bucket.Name(), projectID),
		fmt.Sprintf("DELETE FROM `%s` WHERE doc_type = \"project_user\" AND project_id = \"%s\"", c.Bucket.Name(), projectID),
	}

	for _, query := range queries {
		_, err := c.Cluster.Query(query, nil)
		if err != nil {
			return fmt.Errorf("failed to execute delete query: %v", err)
		}
	}

	return nil
}

// TransferProject transfers a project from one user to another
func (c *CouchbaseDriver) TransferProject(ctx context.Context, userId, from, to string) error {
	// Update project ownership in all relevant documents
	query := fmt.Sprintf("UPDATE `%s` SET owner_id = \"%s\" WHERE owner_id = \"%s\" AND project_id = \"%s\"",
		c.Bucket.Name(), to, from, userId)

	_, err := c.Cluster.Query(query, nil)
	return err
}
