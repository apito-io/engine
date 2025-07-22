# MongoDB Driver for Apito Engine

This package implements the `interfaces.ProjectDBInterface` for MongoDB database support in Apito Engine.

## Implementation Status

The MongoDB driver currently implements the following core functionality:

- Basic document operations (create, read, update, delete)
- Collection management
- Project management
- Document querying and counting

Many advanced features like relationships, field manipulations, and certain specialized queries are currently marked as "not implemented" and will return appropriate errors. These features will be implemented in future iterations as needed.

## Configuration

To use the MongoDB driver, make sure to include the MongoDB credentials in your configuration:

```go
driverCredentials := &models.DriverCredentials{
    Engine:   _const.MongoDBDriver,
    Host:     "127.0.0.1",  // MongoDB server host
    Port:     "27017",      // MongoDB server port
    User:     "username",   // MongoDB username
    Password: "password",   // MongoDB password
    Database: "apito_project", // Database name
}
```

## Usage

The MongoDB driver is automatically selected in the `GetProjectDriver` function when the driver engine is set to `_const.MongoDBDriver`:

```go
import (
    "github.com/apito-io/buffers/protobuff"
    "github.com/apito-io/engine/database/project"
    _const "github.com/apito-io/engine/const"
)

func main() {
    driverCredentials := &models.DriverCredentials{
        Engine:   _const.MongoDBDriver,
        Host:     "127.0.0.1",
        Port:     "27017",
        User:     "username",
        Password: "password",
        Database: "apito_project",
    }

    driver, err := project.GetProjectDriver(driverCredentials)
    if err != nil {
        // Handle error
    }

    // Use the driver
    // ...
}
```

## Dependencies

This driver requires the MongoDB Go Driver:

```
go get go.mongodb.org/mongo-driver/mongo
```
