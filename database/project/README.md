# Database Driver Implementation Guide

This guide explains how to implement new database drivers for the Apito Engine project database interface. Each driver must implement the `ProjectDBInterface` to provide consistent database operations across different database systems.

## Table of Contents

1. [Overview](#overview)
2. [Driver Structure](#driver-structure)
3. [Interface Methods Reference](#interface-methods-reference)
4. [Implementation Guidelines](#implementation-guidelines)
5. [Best Practices](#best-practices)
6. [Examples](#examples)
7. [Testing](#testing)

## Overview

The Apito Engine supports multiple database systems through a unified interface (`ProjectDBInterface`). Each database driver implements this interface to provide:

- **Project Management**: Creating, deleting, and transferring projects
- **Schema Management**: Models, fields, indexes, and collections
- **Document Operations**: CRUD operations on project documents
- **Relationship Management**: Handling relations between documents
- **User Management**: Project-specific user operations
- **Data Aggregation**: Counting, querying, and aggregating data
- **Builder Integration**: Connect/disconnect operations for the query builder

### Supported Database Types

Currently implemented drivers:

- **Document Stores**: MongoDB, Firestore, Couchbase
- **Relational**: SQL (PostgreSQL/MySQL), Oracle, MariaDB
- **Column-Family**: Cassandra
- **Key-Value**: BadgerDB, DynamoDB

## Driver Structure

Each database driver should follow this standard structure:

```
/your-database-driver/
├── init.go          # Driver initialization and connection
├── functions.go     # Core CRUD operations
├── misc.go         # Helper functions and utilities
└── aggregation.go  # (Optional) Complex aggregation functions
```

### Basic Driver Structure

```go
package yourdb

import (
    "context"
    "github.com/apito-io/engine/models"
    "github.com/apito-io/types"
    // Your database client imports
)

// YourDBDriver implements ProjectDBInterface
type YourDBDriver struct {
    Client           *YourDBClient
    Database         string
    DriverCredential *models.DriverCredentials
    // Add other driver-specific fields
}

// GetYourDBDriver creates and initializes a new driver instance
func GetYourDBDriver(driverCredentials *models.DriverCredentials) (*YourDBDriver, error) {
    // Implementation details
}
```

## Interface Methods Reference

The `ProjectDBInterface` contains **47 methods** that must be implemented. Here's a detailed breakdown:

### Project Management (2 methods)

#### `DeleteProject(ctx context.Context, projectID string) error`

- **Purpose**: Completely removes a project and all its data
- **Implementation**: Delete all collections, documents, relations, and metadata for the project
- **Returns**: Error if deletion fails

#### `TransferProject(ctx context.Context, userId, from, to string) error`

- **Purpose**: Transfers project ownership from one user to another
- **Implementation**: Update project ownership metadata and access permissions
- **Parameters**:
  - `userId`: The current user performing the transfer
  - `from`: Current owner ID
  - `to`: New owner ID

### Collection Management (2 methods)

#### `CheckCollectionExists(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) (bool, error)`

- **Purpose**: Verifies if a collection exists in the database
- **Parameters**:
  - `param`: Contains project and model information
  - `isRelationCollection`: True if checking for a relation collection
- **Returns**: Boolean indicating existence and any error

#### `AddCollection(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) error`

- **Purpose**: Creates a new collection in the database
- **Implementation**:
  - For document DBs: Create the collection
  - For SQL DBs: Create the table with appropriate schema
  - For key-value DBs: Set up key prefixes and metadata

### Model Management (5 methods)

#### `AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error)`

- **Purpose**: Adds a new model to the project schema
- **Implementation**:
  - Create collection/table for the model
  - Set up initial schema structure
  - Update project metadata
- **Returns**: Updated project schema

#### `AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, parent_field string) (*models.ModelType, error)`

- **Purpose**: Adds or updates fields in an existing model
- **Parameters**:
  - `isUpdate`: True if updating existing field, false if adding new
  - `parent_field`: For nested/repeated field groups
- **Implementation**: Modify schema structure, handle data migration if needed

#### `RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error`

- **Purpose**: Renames a model and updates all references
- **Implementation**:
  - Rename collection/table
  - Update relation references
  - Update project schema

#### `ConvertModel(ctx context.Context, project *models.Project, modelName string) error`

- **Purpose**: Converts model type or structure
- **Implementation**: Database-specific conversion logic

#### `DropModel(ctx context.Context, project *models.Project, modelName string) error`

- **Purpose**: Completely removes a model and all its data
- **Implementation**: Drop collection/table and clean up relations

### Index Management (2 methods)

#### `CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, parent_field string) error`

- **Purpose**: Creates database indexes for improved query performance
- **Implementation**:
  - Document DBs: Create field indexes
  - SQL DBs: Create B-tree or other appropriate indexes
  - Key-value DBs: May be no-op depending on capabilities

#### `DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error`

- **Purpose**: Removes database indexes
- **Implementation**: Drop the specified index

### Relationship Management (8 methods)

#### `AddRelationFields(ctx context.Context, from *models.ConnectionType, to *models.ConnectionType) error`

- **Purpose**: Creates relation structures between models
- **Implementation**:
  - Document DBs: Set up reference fields or junction collections
  - SQL DBs: Create foreign keys or junction tables
  - Key-value DBs: Set up relation key patterns

#### `DeleteRelationDocuments(ctx context.Context, projectId string, from *models.ConnectionType, to *models.ConnectionType) error`

- **Purpose**: Removes all relation data between two models
- **Implementation**: Delete pivot tables, relation collections, or relation keys

#### `GetRelationDocument(ctx context.Context, param *models.ConnectDisconnectParam) (*models.EdgeRelation, error)`

- **Purpose**: Retrieves a specific relation document
- **Returns**: EdgeRelation containing the relation data

#### `CreateRelation(ctx context.Context, projectId string, relation *models.EdgeRelation) error`

- **Purpose**: Creates a new relation between documents
- **Implementation**: Insert relation data in appropriate structure

#### `DeleteRelation(ctx context.Context, param *models.ConnectDisconnectParam, id string) error`

- **Purpose**: Removes a specific relation by ID
- **Implementation**: Delete the relation record

#### `NewInsertableRelations(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error)`

- **Purpose**: Finds documents that can be related but aren't yet
- **Returns**: Array of document IDs that can be connected

#### `CheckOneToOneRelationExists(ctx context.Context, param *models.ConnectDisconnectParam) (bool, error)`

- **Purpose**: Validates one-to-one relation constraints
- **Returns**: True if relation already exists (preventing duplicates)

#### `GetRelationIds(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error)`

- **Purpose**: Gets all document IDs related to a specific document
- **Returns**: Array of related document IDs

### Builder Integration (2 methods)

#### `ConnectBuilder(ctx context.Context, param *models.CommonSystemParams) error`

- **Purpose**: Handles builder connection operations
- **Implementation**: Database-specific connection logic for the query builder

#### `DisconnectBuilder(ctx context.Context, param *models.CommonSystemParams) error`

- **Purpose**: Handles builder disconnection operations
- **Implementation**: Cleanup and disconnection logic

### User Management (3 methods)

#### `GetProjectUser(ctx context.Context, phone, email, projectId string) (*types.DefaultDocumentStructure, error)`

- **Purpose**: Retrieves user profile by phone/email for a specific project
- **Returns**: User document structure

#### `GetLoggedInProjectUser(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error)`

- **Purpose**: Gets the currently logged-in user's profile for the project
- **Returns**: Current user's document structure

#### `GetProjectUsers(ctx context.Context, projectId string, keys []string) (map[string]*types.DefaultDocumentStructure, error)`

- **Purpose**: Batch retrieval of multiple user profiles
- **Returns**: Map of user ID to user document

### Document Operations (13 methods)

#### `GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error)`

- **Purpose**: Retrieves a single document as raw bytes
- **Returns**: Document serialized as bytes (usually JSON)

#### `GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error)`

- **Purpose**: Retrieves a single document as structured data
- **Returns**: Document as DefaultDocumentStructure

#### `GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*models.DocumentRevisionHistory, error)`

- **Purpose**: Gets the revision history of a document
- **Returns**: Array of document revisions with timestamps

#### `GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error)`

- **Purpose**: Retrieves raw document data without processing
- **Returns**: Raw document data as interface{}

#### `QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error)`

- **Purpose**: Queries multiple documents and returns as bytes
- **Implementation**: Apply filters, pagination, sorting from ResolveParams
- **Returns**: Array of documents serialized as bytes

#### `QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error)`

- **Purpose**: Queries multiple documents and returns as structured data
- **Implementation**: Same as above but returns structured data

#### `AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error)`

- **Purpose**: Creates a new document in the project
- **Implementation**:
  - Generate UUID for document ID
  - Set created/updated timestamps
  - Insert document with proper structure
- **Returns**: Created document ID or full document

#### `UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error`

- **Purpose**: Updates an existing document
- **Parameters**:
  - `replace`: True for full replacement, false for partial update
- **Implementation**: Update document and set updated timestamp

#### `DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error`

- **Purpose**: Deletes a single document
- **Implementation**: Remove document and clean up relations

#### `DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error`

- **Purpose**: Batch deletes multiple documents
- **Implementation**: Apply filters and delete matching documents

#### `DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error`

- **Purpose**: Removes all relations for a specific document
- **Implementation**: Clean up all relation data pointing to the document

#### `GetAllRelationDocumentsOfSingleDocument(ctx context.Context, from string, arg *models.CommonSystemParams) (interface{}, error)`

- **Purpose**: Gets all related documents for a specific document
- **Parameters**: `from` is the source document ID
- **Returns**: Related documents (object or array depending on relation type)

### Data Loading and Relationships (2 methods)

#### `RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) (interface{}, error)`

- **Purpose**: Loads relationship data based on connection parameters
- **Parameters**: `connection` contains relation configuration
- **Returns**: Related data as interface{}

#### `RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) ([]byte, error)`

- **Purpose**: Same as above but returns data as bytes
- **Returns**: Related data serialized as bytes

### Counting Operations (3 methods)

#### `CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error)`

- **Purpose**: Counts documents with filters applied
- **Returns**: Count result as interface{}

#### `CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error)`

- **Purpose**: Same as above but returns count as bytes
- **Returns**: Count serialized as bytes

#### `CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, previewModel bool) (int, error)`

- **Purpose**: Counts multiple documents with optional preview mode
- **Returns**: Integer count

### Aggregation Operations (2 methods)

#### `AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error)`

- **Purpose**: Performs aggregation operations on project documents
- **Implementation**: Apply grouping, summing, averaging based on parameters
- **Returns**: Aggregation results as interface{}

#### `AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error)`

- **Purpose**: Same as above but returns results as bytes
- **Returns**: Aggregation results serialized as bytes

### Field Management (2 methods)

#### `DropField(ctx context.Context, param *models.CommonSystemParams) error`

- **Purpose**: Removes a field from a model and all its data
- **Implementation**:
  - Update schema structure
  - Remove field data from all documents
  - Drop related indexes

#### `RenameField(ctx context.Context, oldFieldName string, repeatedFieldGroup string, param *models.CommonSystemParams) error`

- **Purpose**: Renames a field in a model and updates all data
- **Implementation**:
  - Update schema structure
  - Rename field in all documents
  - Update indexes if needed

### Metadata Management (1 method)

#### `AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error)`

- **Purpose**: Adds metadata information for team members
- **Implementation**: Database-specific metadata storage
- **Returns**: Updated user documents with metadata

## Implementation Guidelines

### 1. Error Handling

Always provide meaningful error messages:

```go
func (d *YourDriver) SomeMethod(ctx context.Context, param *models.CommonSystemParams) error {
    if param == nil {
        return errors.New("parameter cannot be nil")
    }

    if param.Project == nil {
        return errors.New("project information is required")
    }

    // Database operation
    if err := d.Client.DoSomething(); err != nil {
        return fmt.Errorf("failed to perform operation: %w", err)
    }

    return nil
}
```

### 2. Context Handling

Always respect context cancellation:

```go
func (d *YourDriver) QueryDocuments(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
    // Check if context is already cancelled
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Use context in database operations
    result, err := d.Client.QueryWithContext(ctx, query)
    if err != nil {
        return nil, err
    }

    return result, nil
}
```

### 3. Data Structure Handling

Properly handle the `types.DefaultDocumentStructure`:

```go
// DefaultDocumentStructure contains:
type DefaultDocumentStructure struct {
    ID   string                 `json:"id"`
    Meta *DefaultDocumentMeta   `json:"meta"`
    Data map[string]interface{} `json:"data"`
}

type DefaultDocumentMeta struct {
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### 4. Query Parameter Processing

Handle GraphQL resolve parameters:

```go
func (d *YourDriver) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
    query := BuildBaseQuery(param.Model.Name)

    if param.ResolveParams != nil {
        // Handle where conditions
        if where, ok := param.ResolveParams.Args["where"].(map[string]interface{}); ok {
            query = AddWhereConditions(query, where)
        }

        // Handle pagination
        if limit, ok := param.ResolveParams.Args["limit"].(int); ok {
            query = AddLimit(query, limit)
        }

        if offset, ok := param.ResolveParams.Args["offset"].(int); ok {
            query = AddOffset(query, offset)
        }

        // Handle sorting
        if orderBy, ok := param.ResolveParams.Args["orderBy"].(string); ok {
            query = AddOrderBy(query, orderBy)
        }
    }

    return d.ExecuteQuery(ctx, query)
}
```

## Best Practices

### 1. Connection Management

- Use connection pooling when available
- Implement proper connection cleanup
- Handle connection timeouts gracefully
- Implement health checks

### 2. Schema Handling

- Be flexible with schema evolution
- Handle missing fields gracefully
- Implement proper data migration strategies
- Validate data types when possible

### 3. Performance Optimization

- Implement proper indexing strategies
- Use bulk operations when possible
- Optimize query performance
- Implement caching where appropriate

### 4. Security

- Validate all input parameters
- Implement proper access controls
- Prevent injection attacks
- Use parameterized queries

### 5. Transaction Management

For databases that support transactions:

```go
func (d *YourDriver) ComplexOperation(ctx context.Context, param *models.CommonSystemParams) error {
    tx, err := d.Client.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback() // Will be ignored if tx.Commit() succeeds

    // Perform multiple operations
    if err := d.Operation1(ctx, tx, param); err != nil {
        return err
    }

    if err := d.Operation2(ctx, tx, param); err != nil {
        return err
    }

    return tx.Commit()
}
```

## Examples

### Document Database Implementation (MongoDB-style)

```go
func (m *MongoDriver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {
    collection := m.Database.Collection(utility.SingularResourceName(param.Model.Name))

    // Ensure document has proper structure
    if doc.ID == "" {
        doc.ID = uuid.New().String()
    }

    if doc.Meta == nil {
        doc.Meta = &types.DefaultDocumentMeta{
            CreatedAt: time.Now(),
            UpdatedAt: time.Now(),
        }
    }

    // Insert document
    _, err := collection.InsertOne(ctx, doc)
    if err != nil {
        return nil, fmt.Errorf("failed to insert document: %w", err)
    }

    return doc.ID, nil
}
```

### Relational Database Implementation (SQL-style)

```go
func (s *SQLDriver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {
    tableName := utility.SingularResourceName(param.Model.Name)

    // Prepare document data
    if doc.ID == "" {
        doc.ID = uuid.New().String()
    }

    dataBytes, err := json.Marshal(doc.Data)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal document data: %w", err)
    }

    // Insert into SQL table
    query := fmt.Sprintf(`
        INSERT INTO %s (id, data, created_at, updated_at)
        VALUES (?, ?, ?, ?)
    `, tableName)

    now := time.Now()
    _, err = s.DB.ExecContext(ctx, query, doc.ID, string(dataBytes), now, now)
    if err != nil {
        return nil, fmt.Errorf("failed to insert document: %w", err)
    }

    return doc.ID, nil
}
```

### Key-Value Database Implementation (BadgerDB-style)

```go
func (b *BadgerDriver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {
    // Generate key
    if doc.ID == "" {
        doc.ID = uuid.New().String()
    }

    key := fmt.Sprintf("project:%s:model:%s:doc:%s",
        param.Project.ID, param.Model.Name, doc.ID)

    // Ensure proper metadata
    if doc.Meta == nil {
        doc.Meta = &types.DefaultDocumentMeta{
            CreatedAt: time.Now(),
            UpdatedAt: time.Now(),
        }
    }

    // Serialize document
    docBytes, err := json.Marshal(doc)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal document: %w", err)
    }

    // Store in BadgerDB
    err = b.DB.Update(func(txn *badger.Txn) error {
        return txn.Set([]byte(key), docBytes)
    })

    if err != nil {
        return nil, fmt.Errorf("failed to store document: %w", err)
    }

    return doc.ID, nil
}
```

## Testing

### 1. Unit Tests

Create comprehensive unit tests for each method:

```go
func TestYourDriver_AddDocumentToProject(t *testing.T) {
    driver := setupTestDriver(t)
    defer cleanupTestDriver(t, driver)

    tests := []struct {
        name    string
        param   *models.CommonSystemParams
        doc     *types.DefaultDocumentStructure
        want    interface{}
        wantErr bool
    }{
        {
            name: "successful insertion",
            param: &models.CommonSystemParams{
                Project: &models.Project{ID: "test-project"},
                Model:   &models.ModelType{Name: "TestModel"},
            },
            doc: &types.DefaultDocumentStructure{
                Data: map[string]interface{}{"name": "Test"},
            },
            wantErr: false,
        },
        // Add more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := driver.AddDocumentToProject(context.Background(), tt.param, tt.doc)
            if (err != nil) != tt.wantErr {
                t.Errorf("AddDocumentToProject() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            // Add assertions for the result
        })
    }
}
```

### 2. Integration Tests

Test the driver with real database instances:

```go
func TestYourDriver_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Setup real database connection
    driver := setupRealDriver(t)
    defer cleanupRealDriver(t, driver)

    // Test complete workflows
    testCompleteDocumentWorkflow(t, driver)
    testRelationshipWorkflow(t, driver)
    testSchemaEvolution(t, driver)
}
```

### 3. Benchmark Tests

Ensure performance meets requirements:

```go
func BenchmarkYourDriver_QueryMultiDocument(b *testing.B) {
    driver := setupBenchmarkDriver(b)
    defer cleanupBenchmarkDriver(b, driver)

    param := setupBenchmarkParams()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := driver.QueryMultiDocumentOfProject(context.Background(), param)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## Conclusion

Implementing a new database driver requires careful attention to:

1. **Interface Compliance**: All 47 methods must be implemented
2. **Database Paradigm**: Adapt the generic interface to your database's strengths
3. **Error Handling**: Provide meaningful errors and proper context handling
4. **Performance**: Optimize for your database's characteristics
5. **Testing**: Comprehensive testing ensures reliability

Follow the patterns established by existing drivers and refer to their implementations for guidance. Each database type (document, relational, key-value, column-family) has different optimization strategies, but the interface provides a consistent abstraction layer.

For questions or contributions, please refer to the project documentation or open an issue in the repository.
