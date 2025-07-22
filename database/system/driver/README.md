# System Database Driver Implementation Guide

This document provides comprehensive guidelines for implementing system database drivers in the Apito Engine. It's designed for developers who want to create new database drivers or understand the existing implementations.

## Table of Contents

1. [Overview](#overview)
2. [Interface Requirements](#interface-requirements)
3. [Implementation Patterns](#implementation-patterns)
4. [Database-Specific Implementations](#database-specific-implementations)
5. [Function Categories](#function-categories)
6. [Testing Guidelines](#testing-guidelines)
7. [Best Practices](#best-practices)

## Overview

The Apito Engine uses a plugin-based architecture for database drivers, allowing support for multiple database engines through a unified interface. Each driver must implement the `ApitoSystemDB` interface defined in `interfaces/system_db.go`.

### Supported Database Types

- **BadgerDB**: Embedded key-value store (NoSQL)
- **PostgreSQL/MySQL**: Relational databases (SQL)
- **MongoDB**: Document database (NoSQL)

## Interface Requirements

All system drivers must implement the `ApitoSystemDB` interface. Here are the core method categories:

### Core Interface Methods

```go
type ApitoSystemDB interface {
    // Migration
    RunMigration(ctx context.Context) error

    // Project Management
    CreateProject(ctx context.Context, userId string, project *models.Project) (*models.Project, error)
    UpdateProject(ctx context.Context, project *models.Project, replace bool) error
    GetProject(ctx context.Context, id string) (*models.Project, error)
    GetProjects(ctx context.Context, keys []string) ([]*models.Project, error)
    CheckProjectName(ctx context.Context, name string) error
    SearchProjects(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Project], error)
    FindUserProjects(ctx context.Context, userId string) ([]*models.Project, error)
    FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error)
    DeleteProjectFromSystem(ctx context.Context, projectId string) error
    CheckProjectWithRoles(ctx context.Context, userId, projectId string) (*models.ProjectWithRoles, error)

    // User Management
    CreateSystemUser(ctx context.Context, user *models.SystemUser) (*models.SystemUser, error)
    UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error
    GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error)
    GetSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error)
    GetSystemUsers(ctx context.Context, keys []string) ([]*models.SystemUser, error)
    SearchUsers(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error)
    AddSystemUserMetaInfo(ctx context.Context, doc *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error)

    // Team Management
    GetTeams(ctx context.Context, userId string) ([]*models.Team, error)
    GetTeamsMembers(ctx context.Context, projectId string) ([]*models.SystemUser, error)
    CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error)
    GetProjectTeams(ctx context.Context, projectId string) (*models.Team, error)
    AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error
    RemoveATeamMemberFromProject(ctx context.Context, projectId string, memberID string) error
    AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error)

    // Organization Management
    GetOrganizations(ctx context.Context, userId string) (*models.SearchResponse[models.Organization], error)
    FindOrganizationAdmin(ctx context.Context, orgId string) (*models.SystemUser, error)
    FindUserOrganizations(ctx context.Context, userId string) ([]*models.Organization, error)
    CreateOrganization(ctx context.Context, org *models.Organization) (*models.Organization, error)
    AssignTeamToOrganization(ctx context.Context, orgId, userId, teamId string) error
    RemoveATeamFromOrganization(ctx context.Context, orgId, userId, teamId string) error
    AssignProjectToOrganization(ctx context.Context, orgId, userId, projectId string) error
    RemoveProjectFromOrganization(ctx context.Context, orgId, userId, projectId string) error

    // Function Management
    SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error)

    // Webhook Management
    SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error)
    GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error)
    DeleteWebhook(ctx context.Context, projectId, hookId string) error
    AddWebhookToProject(ctx context.Context, doc *models.Webhook) (*models.Webhook, error)

    // Authentication & Security
    CheckTokenBlacklisted(ctx context.Context, tokenId string) error
    BlacklistAToken(ctx context.Context, token map[string]interface{}) error

    // Audit & Logging
    SaveAuditLog(ctx context.Context, auditLog *models.AuditLogs) error
    SearchAuditLogs(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error)

    // Generic Operations
    SearchResource(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[any], error)
    SaveRawData(ctx context.Context, collection string, data map[string]interface{}) error
}
```

## Implementation Patterns

### 1. Driver Structure

Each driver should be organized in the following file structure:

```
driver/
├── <database>/
│   ├── init.go       # Driver initialization and basic methods
│   ├── functions.go  # Core CRUD and search functions
│   ├── misc.go       # Organization and team management
│   └── README.md     # Database-specific documentation (optional)
```

### 2. Error Handling

Implement consistent error handling patterns:

```go
func (d *Driver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
    // Always check for empty/invalid parameters
    if id == "" {
        return nil, fmt.Errorf("user ID cannot be empty")
    }
    
    // Database-specific operations
    // ...
    
    // Handle not found cases consistently
    if err == database.ErrNotFound {
        return nil, fmt.Errorf("user not found with ID: %s", id)
    }
    
    return user, err
}
```

### 3. Context Management

Always respect context for cancellation and timeouts:

```go
func (d *Driver) SomeOperation(ctx context.Context, param string) error {
    // Use context in all database operations
    result, err := d.db.QueryContext(ctx, query, param)
    if err != nil {
        return err
    }
    defer result.Close()
    
    // Check for context cancellation in loops
    for result.Next() {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Process result
        }
    }
    
    return nil
}
```

### 4. Comments and Documentation

Every function must have a comment explaining its purpose:

```go
// GetSystemUser retrieves a system user by ID using [DatabaseType]
// Returns an error if the user is not found or if a database error occurs
func (d *Driver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
    // Implementation...
}
```

## Database-Specific Implementations

### BadgerDB (Key-Value Store)

**Key Patterns:**
- Users: `user:{id}`
- Projects: `project:{id}`
- User-Project Relations: `user_project:{userId}:{projectId}`
- Organizations: `org:{id}`
- Teams: `team:{id}`
- Webhooks: `webhook:{projectId}:{hookId}`
- Token Blacklist: `blacklist:{tokenId}`
- Audit Logs: `audit:{id}`

**Implementation Example:**
```go
func (b *BadgerDriver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
    var user models.SystemUser
    err := b.Db.View(func(txn *badger.Txn) error {
        item, err := txn.Get([]byte("user:" + id))
        if err != nil {
            return err
        }
        
        err = item.Value(func(val []byte) error {
            return json.Unmarshal(val, &user)
        })
        return err
    })
    
    return &user, err
}
```

### SQL Databases (PostgreSQL/MySQL)

**Table Structure:**
- `users` - System users
- `projects` - Projects
- `organizations` - Organizations
- `teams` - Teams
- `user_projects` - User-project relationships
- `user_organizations` - User-organization relationships
- `project_teams` - Project-team relationships
- `webhooks` - Webhook configurations
- `token_blacklist` - Blacklisted tokens
- `audit_logs` - Audit trail

**Implementation Example:**
```go
func (p *PostgreSQLDriver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
    user := &models.SystemUser{}
    err := p.ORM.NewSelect().
        Model(user).
        Where("id = ?", id).
        Scan(ctx)
    if err != nil {
        return nil, err
    }
    return user, nil
}
```

### MongoDB (Document Database)

**Collection Structure:**
- `users` - System users
- `projects` - Projects  
- `organizations` - Organizations
- `teams` - Teams
- `user_projects` - User-project relationships
- `user_organizations` - User-organization relationships
- `project_teams` - Project-team relationships
- `webhooks` - Webhook configurations
- `token_blacklist` - Blacklisted tokens
- `audit_logs` - Audit trail

**Implementation Example:**
```go
func (m *SystemMongoDriver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
    collection := m.Database.Collection("users")
    
    var user models.SystemUser
    err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
    if err != nil {
        return nil, err
    }
    
    return &user, nil
}
```

## Function Categories

### 1. Migration Functions

**Purpose:** Initialize database schema and create necessary structures.

**Implementation Requirements:**
- Create tables/collections/keyspaces as needed
- Set up indexes for performance
- Handle existing structures gracefully
- Return errors for critical failures only

### 2. CRUD Operations

**Purpose:** Basic Create, Read, Update, Delete operations for core entities.

**Implementation Requirements:**
- Generate UUIDs for new entities if ID is empty
- Set `created_at` and `updated_at` timestamps
- Handle both replace and partial updates
- Validate required fields

### 3. Search and Query Functions

**Purpose:** Complex queries with filtering, pagination, and sorting.

**Implementation Requirements:**
- Support common system parameters
- Implement reasonable default limits (e.g., 100 records)
- Handle empty result sets gracefully
- Optimize queries with proper indexes

### 4. Relationship Management

**Purpose:** Manage relationships between users, projects, teams, and organizations.

**Implementation Requirements:**
- Maintain referential integrity
- Support role-based access control
- Handle cascade deletions appropriately
- Validate relationship constraints

### 5. Security Functions

**Purpose:** Handle authentication, authorization, and audit trails.

**Implementation Requirements:**
- Secure token blacklisting
- Comprehensive audit logging
- Permission validation
- Protect against injection attacks

## Testing Guidelines

### Unit Testing

Create comprehensive unit tests for each driver:

```go
func TestDriverGetSystemUser(t *testing.T) {
    driver := setupTestDriver(t)
    defer cleanupTestDriver(t, driver)
    
    // Test cases
    testCases := []struct {
        name     string
        userID   string
        expected *models.SystemUser
        hasError bool
    }{
        {
            name:     "Valid user ID",
            userID:   "valid-user-id",
            expected: &models.SystemUser{ID: "valid-user-id"},
            hasError: false,
        },
        {
            name:     "Empty user ID",
            userID:   "",
            expected: nil,
            hasError: true,
        },
        {
            name:     "Non-existent user",
            userID:   "non-existent",
            expected: nil,
            hasError: true,
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            user, err := driver.GetSystemUser(context.Background(), tc.userID)
            
            if tc.hasError {
                assert.Error(t, err)
                assert.Nil(t, user)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tc.expected.ID, user.ID)
            }
        })
    }
}
```

### Integration Testing

Test the driver with real database instances:

```go
func TestIntegrationDriverOperations(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    driver := setupRealDatabase(t)
    defer cleanupRealDatabase(t, driver)
    
    // Test full workflow
    user := &models.SystemUser{
        Email:     "test@example.com",
        FirstName: "Test",
        LastName:  "User",
    }
    
    // Create
    createdUser, err := driver.CreateSystemUser(context.Background(), user)
    assert.NoError(t, err)
    assert.NotEmpty(t, createdUser.ID)
    
    // Read
    retrievedUser, err := driver.GetSystemUser(context.Background(), createdUser.ID)
    assert.NoError(t, err)
    assert.Equal(t, user.Email, retrievedUser.Email)
    
    // Update
    retrievedUser.FirstName = "Updated"
    err = driver.UpdateSystemUser(context.Background(), retrievedUser, false)
    assert.NoError(t, err)
    
    // Delete (if applicable)
    // ...
}
```

## Best Practices

### 1. Performance Optimization

- **Use Indexes:** Create appropriate indexes for frequently queried fields
- **Batch Operations:** Group multiple operations when possible
- **Connection Pooling:** Implement proper connection management
- **Query Optimization:** Use efficient queries specific to each database type

### 2. Error Handling

- **Consistent Errors:** Return meaningful error messages
- **Context Awareness:** Always check for context cancellation
- **Resource Cleanup:** Use defer statements for cleanup
- **Logging:** Log errors with appropriate context

### 3. Security Considerations

- **Input Validation:** Validate all input parameters
- **SQL Injection Prevention:** Use parameterized queries
- **Access Control:** Implement proper permission checks
- **Audit Trails:** Log all sensitive operations

### 4. Code Organization

- **Separation of Concerns:** Keep different functionality in separate files
- **Consistent Naming:** Use clear, descriptive function names
- **Documentation:** Comment all public functions and complex logic
- **Error Propagation:** Handle errors at appropriate levels

### 5. Database-Specific Optimizations

#### BadgerDB
- Use transactions for batch operations
- Implement proper key prefixing for organization
- Consider TTL for temporary data
- Use iterators efficiently

#### SQL Databases
- Utilize ORM features properly
- Implement proper transaction handling
- Use prepared statements
- Consider database-specific optimizations

#### MongoDB
- Use appropriate indexes
- Implement proper aggregation pipelines
- Handle large result sets with cursors
- Use transactions for consistency

## Creating a New Driver

To create a new database driver:

1. **Create Directory Structure:**
   ```bash
   mkdir -p open-core/database/system/driver/newdb
   cd open-core/database/system/driver/newdb
   ```

2. **Implement Base Files:**
   - `init.go` - Driver struct and initialization
   - `functions.go` - Core CRUD operations
   - `misc.go` - Organization and team management

3. **Define Driver Struct:**
   ```go
   type NewDBDriver struct {
       Client     *newdb.Client
       Database   *newdb.Database
       // Add other necessary fields
   }
   ```

4. **Implement Interface Methods:**
   Start with basic operations and gradually implement all interface methods.

5. **Add Tests:**
   Create comprehensive unit and integration tests.

6. **Update Factory:**
   Add the new driver to the driver factory function.

7. **Document:**
   Create specific documentation for the new driver.

## Conclusion

This guide provides the foundation for implementing system database drivers in the Apito Engine. Each driver must implement the complete `ApitoSystemDB` interface while following database-specific best practices. Comprehensive testing and documentation are essential for maintaining code quality and helping future developers understand and extend the system.

For questions or clarifications, refer to the existing implementations in the `badger/`, `sql/`, and `mongodb/` directories as reference examples. 