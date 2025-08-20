# BBolt System Driver for Apito Engine

This is a BBolt database driver implementation for the Apito Engine system. It uses the [ApitoBolt SDK](https://github.com/apito-io/apitoBolt) v0.1.1 to provide a MongoDB-like interface on top of BBolt for system-level operations.

## Features

The BBolt driver implements the complete `ApitoSystemDB` interface and provides:

### User Management

- Create, update, and retrieve system users
- User authentication and authorization support
- User search functionality

### Project Management

- Create, update, and delete projects
- Project search and filtering
- User-project role assignments
- Project schema and settings management

### Team & Organization Management

- Team creation and member management
- Organization structure support
- Role-based permissions

### Webhooks

- Webhook creation and management
- Project-specific webhook filtering
- Webhook deletion and updates

### Audit Logging

- Comprehensive audit trail
- Activity logging with user and project context
- Searchable audit logs

### Usage Tracking

- Project usage metrics
- Bandwidth and API call tracking
- Monthly usage reports

### Token Management

- Token blacklisting
- Security token validation

### Subscription & Billing (Pro Features)

- Subscription management
- Invoice creation and tracking
- Usage-based billing support

## Architecture

The driver follows the Apito Engine's dual plugin architecture pattern:

- **Core Implementation**: Implements the `ApitoSystemDB` interface from `open-core/interfaces/system_db.go`
- **Pro Extensions**: Supports additional pro features through the `ProApitoSystemDB` interface
- **BBolt Storage**: Uses ApitoBolt SDK for efficient embedded database operations
- **Collection-Based**: Organizes data into logical collections (users, projects, webhooks, etc.)

## File Structure

```
bbolt/
├── init.go          # Driver initialization and migration
├── functions.go     # Project-related operations
├── query.go         # User and search operations
├── mutation.go      # Data modification operations
├── misc.go          # Helper functions and utilities
├── models.go        # Internal data models
├── driver_test.go   # Test suite
└── README.md        # This documentation
```

## Usage

### Basic Setup

```go
import "gitlab.com/apito.io/engine/database/system/driver/bbolt"

// Create driver credentials
driverCred := &models.DriverCredentials{
    Database: "/path/to/apito_system.db",
}

// Initialize the driver
driver, err := bbolt.GetProSystemBBoltDriver(driverCred, cacheDriver)
if err != nil {
    log.Fatal("Failed to initialize BBolt driver:", err)
}
defer driver.Close()
```

### Example Operations

```go
ctx := context.Background()

// Create a user
user := &models.SystemUser{
    FirstName: "John",
    LastName:  "Doe",
    Email:     "john@example.com",
}
createdUser, err := driver.CreateSystemUser(ctx, user)

// Create a project
project := &models.Project{
    Name:        "My Project",
    Description: "A sample project",
}
createdProject, err := driver.CreateProject(ctx, createdUser.ID, project)

// Search projects
searchParams := &models.CommonSystemParams{
    ProjectID: createdProject.ID,
}
results, err := driver.SearchProjects(ctx, searchParams)
```

## Collections

The driver automatically creates and manages the following collections:

- `users` - System users with email indexing
- `projects` - Projects with organization indexing
- `organizations` - Organization structures
- `teams` - Team definitions
- `team_memberships` - User-project team relationships
- `webhooks` - Project webhooks with project_id indexing
- `audit_logs` - Audit trail with project_id and user_id indexing
- `token_blacklist` - Blacklisted tokens
- `usages` - Usage tracking with project_id indexing
- `subscriptions` - Subscription data with user_id indexing
- `invoices` - Invoice records

## Performance Considerations

- **Embedded Database**: BBolt provides excellent performance for single-node deployments
- **Indexing**: Automatic secondary indexes on frequently queried fields
- **Memory Efficient**: Low memory footprint compared to full database systems
- **ACID Compliance**: Full ACID transaction support through BBolt
- **File-Based**: Single file database for easy backup and deployment

## Limitations

- **Single Node Only**: BBolt is designed for single-node deployments
- **Read/Write Locks**: Write operations are serialized (but this is usually fine for system operations)
- **Simplified Queries**: Some complex query operations are simplified compared to full SQL databases
- **No Built-in Replication**: For high availability, consider using external replication tools

## Testing

Run the test suite:

```bash
cd database/system/driver/bbolt
go test -v
```

The tests cover:

- Basic CRUD operations
- User and project management
- Search functionality
- Audit logging
- Database migration

## Dependencies

- [ApitoBolt](https://github.com/apito-io/apitoBolt) v0.1.1 - BBolt wrapper with MongoDB-like API
- [BBolt](https://github.com/etcd-io/bbolt) - Underlying embedded key-value database
- Standard Apito Engine models and interfaces

## Migration from Other Drivers

The BBolt driver implements the same interface as other system drivers (ArangoDB, PostgreSQL, etc.), making migration straightforward:

1. Update driver configuration to use BBolt
2. Run migration to create collections and indexes
3. Optionally migrate existing data using export/import tools

## Configuration

### Database Path

- Default: `apito_system.db` in current directory
- Configurable via `DriverCredentials.Database` field
- Supports both relative and absolute paths

### Caching

- Supports external cache drivers via `CacheDBInterface`
- Cache integration for improved performance
- Optional - can operate without caching

## Monitoring and Maintenance

### Database Statistics

```go
stats, err := driver.GetDatabaseStats()
// Returns collection counts and basic metrics
```

### Backup

```go
err := driver.BackupDatabase("/path/to/backup.db")
// Note: Full backup implementation pending
```

### Health Checks

The driver automatically validates database integrity during initialization and provides error reporting for any issues.

## Contributing

When contributing to the BBolt driver:

1. Follow the existing code patterns
2. Add tests for new functionality
3. Update documentation for interface changes
4. Ensure compatibility with the `ApitoSystemDB` interface
5. Consider performance implications of new features

## License

This driver is part of the Apito Engine and follows the same license terms.
