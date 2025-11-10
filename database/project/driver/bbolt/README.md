# BBolt Project Driver for Apito Engine

This is a BBolt database driver implementation for the Apito Engine project database. It uses the [ApitoBolt SDK](https://github.com/apito-io/apitoBolt) to provide a MongoDB-like interface on top of BBolt for project-level operations.

## Features

The BBolt driver implements the complete `ProjectDBInterface` and provides:

### Project Management

- Delete projects and all associated data
- Transfer project ownership between users

### Collection Management

- Check collection existence
- Add collections to projects

### Model Management

- Add/update models to projects
- Add fields to models
- Rename models
- Drop models from projects

### Index Management

- Create indexes on model fields
- Drop indexes (lightweight in ApitoBolt)

### Document Operations

- Full CRUD operations on documents
- Query multiple documents
- Document revision tracking
- Count documents
- Aggregate document data

### Relation Management

- Create relations between documents
- Delete relations
- Query relations
- Check one-to-one relation existence
- Get relation IDs

### Builder Integration

- Connect/disconnect builders to projects

### User Management

- Project-specific user operations
- Get logged-in project user
- Get multiple project users

## Architecture

The driver uses ApitoBolt's collection-based API with the following collections:

- `project_docs` - Main documents collection
- `project_relations` - Relations/edges between documents
- `project_revisions` - Document revision history
- `project_models` - Model metadata
- `project_builders` - Builder connections
- `project_users` - Project-specific user data
- `project_metadata` - Project metadata

## Data Models

### ProjectDocument

Represents a document in a project with fields:

- ID, ProjectID, ModelName
- Data (map of fields)
- CreatedAt, UpdatedAt timestamps

### ProjectRelation

Represents a relation between documents:

- ID, ProjectID
- FromID, ToID
- RelationName, RelationType
- Data (additional fields)

### ProjectRevision

Document revision history:

- ID, ProjectID, DocumentID
- Version number
- Data snapshot
- CreatedBy, CreatedAt

## Usage

```go
import "github.com/apito-io/engine/database/project/driver/bbolt"

// Create driver instance
driver, err := bbolt.GetBBoltDriver(driverCredentials)
if err != nil {
    // Handle error
}
defer driver.Close()

// Use driver for project operations
doc, err := driver.GetSingleProjectDocument(ctx, param)
```

## Configuration

Set the engine type to "bbolt" or "bolt" in your driver credentials:

```go
driverCred := &models.DriverCredentials{
    Engine:   "bbolt",
    Database: "/path/to/project.db", // Optional, defaults to ~/.apito/db/apito_project.db
}
```

## Benefits vs Badger

- **Simpler API**: MongoDB-like collection operations
- **Built-in Indexing**: ApitoBolt provides secondary indexes
- **Type Safety**: Struct-based data models
- **ACID Transactions**: Full transaction support
- **Smaller Footprint**: Single file database
- **Better Performance**: Optimized for read-heavy workloads

## Limitations

- **Single Node**: BBolt is designed for single-node deployments
- **Write Serialization**: Writes are serialized (but reads are concurrent)
- **No Built-in Replication**: Use external tools for replication

## Testing

```bash
cd open-core/database/project/driver/bbolt
go test -v
```

## Dependencies

- [ApitoBolt](https://github.com/apito-io/apitoBolt) v0.1.1+
- Standard Apito Engine models and interfaces
