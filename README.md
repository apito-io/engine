<p align="center">
  <img src="https://raw.githubusercontent.com/apito-io/engine/main/docs/cover-photo.png" alt="Apito Logo" />
</p>

# Apito Engine

**Build locally. Scale when you’re ready.**

Apito Engine is a self-hosted GraphQL stack. You run it on infrastructure you control. When you want managed hosting, sync the same project to Apito Cloud.

Licensed under AGPL-3.0. You can use, modify and self-host it. If you run a modified version as a network service, you must publish your changes under AGPL-3.0. A commercial license is available from Apito LLC. Releases v1.8.26 to v1.8.30 remain FSL-1.1-ALv2; see LICENSING.md.

The self-hosted binary is `github.com/apito-io/open_driver/cmd/oss`. It injects the public database, cache, key-value, and in-process realtime factories. `go run` in this module alone does not inject those factories. Plugins and a networked realtime bus are optional and stay out of the default binary.

<p align="center">
  <a href="https://apito.io"><strong>Website</strong></a> ·
  <a href="https://apito.io/docs"><strong>Documentation</strong></a> ·
  <a href="https://discord.gg/Xb7p2ddC6E"><strong>Discord</strong></a>
</p>

<p align="center">
  <a href="https://github.com/apito-io/engine/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/license-AGPL--3.0-blue.svg" alt="AGPL-3.0" />
  </a>
  <a href="https://github.com/apito-io/engine/releases">
    <img src="https://img.shields.io/github/v/release/apito-io/engine" alt="Release" />
  </a>
  <a href="https://goreportcard.com/report/github.com/apito-io/engine">
    <img src="https://goreportcard.com/badge/github.com/apito-io/engine" alt="Go Report Card" />
  </a>
  <a href="https://github.com/apito-io/engine/actions">
    <img src="https://github.com/apito-io/engine/workflows/Build%20and%20Release/badge.svg" alt="Build Status" />
  </a>
</p>

---

## Features

- **GraphQL APIs** - Connect your database and get a GraphQL API. [Docs](https://apito.io/docs)
- **Plugins** - Extend the engine with Go plugins and process-isolated HashiCorp plugins. [Docs](https://apito.io/docs/developing-plugins/first-plugin/)
- **Databases** - SQLite, PostgreSQL, MySQL, MariaDB, and MongoDB. DynamoDB, Oracle, and Cassandra are coming soon. [Docs](https://apito.io/docs/platform/database-design/)
- **Authentication** - JWT auth, role-based access control, API keys, and multi-tenant isolation. [Docs](https://apito.io/docs/platform/auth/)
- **Subscriptions** - WebSocket GraphQL subscriptions. [Docs](https://apito.io/docs)
- **Functions** - Function execution inside GraphQL queries and mutations. [Docs](https://apito.io/docs/platform/functions/)
- **File Storage** - File upload and management. [Docs](https://apito.io/docs/platform/files/)
- **Multi-tenant SaaS** - Tenant isolation. [Docs](https://apito.io/docs)
- **📊 Admin Dashboard** - Comprehensive dashboard for schema management, data browsing, and analytics.

### 🎯 Apito Console

<table>
<tr>
<td align="center" width="33%">

**📋 Model Management**  
<kbd><img src="https://cdn.apito.io/media/shikho_website_v2_8ma77/30RCY24Q15_Apito_Console___Model.png" alt="Apito Console - Model" width="100%"/></kbd>  
_Design your data models with an intuitive visual editor_

</td>
<td align="center" width="33%">

**📝 Content Management**  
<kbd><img src="https://cdn.apito.io/media/shikho_website_v2_8ma77/1ZGH5QE8U5_Apito_Console___Content.png" alt="Apito Console - Content" width="100%"/></kbd>  
_Manage your content with a powerful admin interface_

</td>
<td align="center" width="33%">

**🚀 API Explorer**  
<kbd><img src="https://cdn.apito.io/media/shikho_website_v2_8ma77/IN1TXKJPX5_Apito_Console___API.png" alt="Apito Console - API" width="100%"/></kbd>  
_Test and explore your GraphQL APIs in real-time_

</td>
</tr>
</table>

> **Watch "releases"** of this repo to get notified of major updates.

## Quick Start

### Install

```bash
curl -fsSL https://get.apito.io/install.sh | bash
apito start
```

The console is at http://localhost:4055. The engine API is at http://localhost:5055.

`apito start` pulls `ghcr.io/apito-io/studio`.

### From Source

The public binary is `github.com/apito-io/open_driver/cmd/oss`. `go run` in this module alone does not inject the database, cache, key-value, and realtime factories.

```bash
git clone https://github.com/apito-io/engine
git clone https://github.com/apito-io/open_driver
cd open_driver/cmd/oss
go mod edit -replace github.com/apito-io/engine=/path/to/engine
go mod tidy
go run .
```

### Pre-built binaries

Linux release assets are `apito-engine-linux-amd64` and `apito-engine-linux-arm64` on [GitHub Releases](https://github.com/apito-io/engine/releases).

## How it works

open-core is a GraphQL-first stack that combines proven open-source technologies with a plugin architecture. It is designed to be extended through plugins while keeping performance and security in focus.

### Architecture

open-core uses a modern, cloud-native layout built for scalability:

```mermaid
graph TB
    Client[🌐 Client Applications<br/>Web, Mobile, Desktop]

    subgraph "🔥 Apito open-core"
        Router[🌐 Echo Router<br/>HTTP/WebSocket Server]
        Auth[🔐 Auth Service<br/>JWT + API Keys]
        GraphQL[🧠 GraphQL layer<br/>Schema + Resolvers]
        Realtime[⚡ Real-time layer<br/>WebSocket Subscriptions]
        Storage[📁 Storage Service<br/>File Management]

        subgraph "🔌 Plugin System"
            GoPlugins[⚡ Go Plugins<br/>High Performance<br/>.so files]
            HCPlugins[🛡️ HashiCorp Plugins<br/>Process Isolated<br/>Cross-language]
        end

        subgraph "🗄️ Database Layer"
            DBDrivers[Database Drivers<br/>Connection Pooling]
            MongoDB[(🟠 MongoDB)]
            PostgreSQL[(🐘 PostgreSQL)]
            MySQL[(🐬 MySQL)]
            MariaDB[(🦭 MariaDB)]
            SQLite[(📄 SQLite)]
        end

        subgraph "💾 Cache Layer"
            Redis[(🔴 Redis<br/>Cache & Sessions)]
            KVStore[Key-Value Store]
        end
    end

    subgraph "🔧 External Services"
        S3[☁️ AWS S3<br/>File Storage]
        Monitoring[📊 Monitoring<br/>Sentry, Metrics]
        CDN[🌍 CDN<br/>Static Assets]
    end

    %% Client to open-core
    Client --> Router

    %% Router routing
    Router --> Auth
    Router --> GraphQL
    Router --> Realtime
    Router --> Storage

    %% Auth flow
    Auth --> Redis

    %% GraphQL to plugins
    GraphQL --> GoPlugins
    GraphQL --> HCPlugins

    %% Database connections
    GoPlugins --> DBDrivers
    HCPlugins --> DBDrivers
    GraphQL --> DBDrivers

    %% Database types
    DBDrivers --> MongoDB
    DBDrivers --> PostgreSQL
    DBDrivers --> MySQL
    DBDrivers --> MariaDB
    DBDrivers --> SQLite
    DBDrivers --> SQLite

    %% Cache connections
    GraphQL --> Redis
    Auth --> KVStore

    %% External services
    Storage --> S3
    Router --> Monitoring
    Storage --> CDN

    %% Styling
    classDef openCoreNode fill:#ff6b6b,stroke:#ffffff,stroke-width:2px,color:#ffffff
    classDef database fill:#4ecdc4,stroke:#ffffff,stroke-width:2px,color:#ffffff
    classDef plugin fill:#ffe66d,stroke:#ffffff,stroke-width:2px,color:#000000
    classDef external fill:#a8e6cf,stroke:#ffffff,stroke-width:2px,color:#000000

    class Router,Auth,GraphQL,Realtime,Storage openCoreNode
    class MongoDB,PostgreSQL,MySQL,MariaDB,SQLite database
    class GoPlugins,HCPlugins plugin
    class S3,Monitoring,CDN external
```

**Core Components:**

- **🧠 GraphQL layer** - Built with [`graphql-go/graphql`](https://github.com/graphql-go/graphql) for dynamic schema generation and high-performance query execution
- **🌐 Echo Router** - High-performance HTTP router with middleware for REST endpoints and GraphQL over HTTP
- **🔌 Plugin System** - Dual architecture supporting both Go built-in plugins (`.so` files) and HashiCorp process-isolated plugins
- **Database drivers** - SQLite, PostgreSQL, MySQL, MariaDB, and MongoDB, with connection pooling and transactions
- **🔐 Auth Service** - JWT-based authentication with RSA signing, API keys, and session management
- **⚡ Real-time layer** - WebSocket-based subscriptions for live data updates
- **📁 Storage Service** - File upload and management with multiple backend support

### Plugin Architecture

The dual plugin system combines:

#### Go Built-in Plugins (High Performance)

- **Shared Memory** - Direct memory access for maximum performance
- **Type Safety** - Compile-time type checking and Go's strong typing
- **Fast Execution** - No serialization overhead for critical operations

```go
// Example Go plugin
package main

import "github.com/apito-io/engine/interfaces"

var MyPlugin interfaces.NormalPluginInterface = &UploadPlugin{}

type UploadPlugin struct{}

func (p *UploadPlugin) ExecuteFunction(params map[string]interface{}) (interface{}, error) {
    // High-performance plugin logic
    return result, nil
}
```

#### HashiCorp Plugins (Security & Isolation)

- **Process Isolation** - Plugins run in separate processes for security
- **Crash Recovery** - Automatic restart on plugin failures
- **Language Agnostic** - Support for Go, JavaScript, Python, and more

```go
// Example HashiCorp plugin
func main() {
    plugin := &MyHashiCorpPlugin{}

    hcplugin.Serve(&hcplugin.ServeConfig{
        HandshakeConfig: plugins.HandshakeConfig,
        Plugins: map[string]hcplugin.Plugin{
            "my_plugin": &plugins.HashiCorpPluginRPC{Impl: plugin},
        },
        GRPCServer: hcplugin.DefaultGRPCServer,
    })
}
```

## Client Libraries

| Language | Admin SDK | Plugin SDK |
| --- | --- | --- |
| JavaScript | [js-admin-sdk](https://github.com/apito-io/js-admin-sdk) | [js-plugin-build-sdk](https://github.com/apito-io/js-plugin-build-sdk) |
| Go | [go-admin-sdk](https://github.com/apito-io/go-internal-sdk) | [go-apito-plugin-sdk](https://github.com/apito-io/go-apito-plugin-sdk) |
| Flutter | [flutter_admin_sdk](https://github.com/apito-io/flutter_admin_sdk) | |

## Examples

### Creating Your First API

```bash
# Using the Apito CLI
apito create project -n fitnessApp

# Connect your database
apito db connect --engine postgresql --host localhost --port 5432

# Generate GraphQL schema from existing tables
apito schema generate

# Deploy locally
apito dev
```

### GraphQL Query Example

```graphql
# Query with real-time subscription
subscription {
  users(filter: { status: "active" }) {
    id
    name
    email
    posts {
      title
      createdAt
    }
  }
}

# Mutation with custom function
mutation {
  uploadUserAvatar(userId: "123", file: $file) {
    url
    thumbnails
    metadata
  }
}
```

### Plugin Development

Create custom functionality with minimal code:

```go
// main.go - Simple function plugin
package main

import (
    sdk "github.com/apito-io/go-apito-plugin-sdk"
)

func main() {
    // Initialize plugin
    plugin := sdk.Init("image-processor", "1.0.0", "your-api-key")

    // Register a custom function
    plugin.RegisterFunction("processImage", func(args map[string]interface{}) (interface{}, error) {
        imageUrl := args["url"].(string)
        // Process image logic here
        return map[string]interface{}{
            "processedUrl": processedUrl,
            "thumbnails": thumbnails,
        }, nil
    })

    // Register GraphQL mutation
    plugin.RegisterMutation("uploadImage", map[string]interface{}{
        "type": "ImageUploadResult",
        "args": map[string]interface{}{
            "file": sdk.NonNull(sdk.Upload),
            "size": sdk.StringArg(),
        },
        "resolve": "processImage",
    })

    // Start plugin
    plugin.Serve()
}
```

## Configuration

Apito can be configured via environment variables or `.env` file:

```env
# Database Configuration (choose one)
DATABASE_ENGINE=sqlite    # sqlite | postgresql | mysql | mariadb | mongodb

# MongoDB Configuration
DATABASE_HOST=localhost     # For Atlas: your-cluster.mongodb.net
DATABASE_PORT=27017         # Leave empty for MongoDB Atlas (uses SRV)
DATABASE_NAME=apito
DATABASE_USERNAME=admin     # Usually 'admin' for MongoDB
DATABASE_PASSWORD=password

# Other Database Ports
# DATABASE_PORT=5432        # PostgreSQL
# DATABASE_PORT=3306        # MySQL/MariaDB

# Server Configuration
PORT=5055
HOST=0.0.0.0
ENV=development

# Authentication
PRIVATE_KEY_PATH=keys/private.key
PUBLIC_KEY_PATH=keys/public.key
JWT_TTL=24h

# Storage
STORAGE_ENGINE=local
STORAGE_PATH=./uploads

# Plugins
PLUGIN_DIR=./plugins
ENABLE_PLUGIN_SYSTEM=true

# Optional Services
REDIS_HOST=localhost
REDIS_PORT=6379
SENTRY_DSN=your-sentry-dsn
```

## Development

### Prerequisites

- Go 1.22 or higher
- Database (MongoDB, PostgreSQL, or MySQL)
- Redis (optional, for caching)

### MongoDB Setup & Troubleshooting

**Common Authentication Issues:**

If you get `auth error: sasl conversation error: unable to authenticate using mechanism "SCRAM-SHA-1"`, try:

1. **Check Username/Password**: Ensure credentials are correct
2. **Use admin database**: Most MongoDB users are created in the `admin` database
3. **Create user properly**:

   ```javascript
   // Connect to MongoDB shell
   mongo

   // Switch to admin database
   use admin

   // Create user with proper roles
   db.createUser({
     user: "admin",
     pwd: "password",
     roles: [
       { role: "userAdminAnyDatabase", db: "admin" },
       { role: "readWriteAnyDatabase", db: "admin" }
     ]
   })
   ```

4. **For MongoDB Atlas**: Use the connection string provided in Atlas dashboard
5. **For local MongoDB**: Ensure authentication is enabled in `mongod.conf`:
   ```yaml
   security:
     authorization: enabled
   ```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/apito-io/engine
cd engine/open-core

# Install dependencies
go mod tidy

# Generate RSA keys for JWT
make keys

# Build the project
make build

# Run tests
make test

# Start development server
make dev
```

### Plugin Development

```bash
# Create a new plugin
mkdir plugins/my-plugin
cd plugins/my-plugin

# Initialize with SDK
go mod init my-plugin
go get github.com/apito-io/go-apito-plugin-sdk

# Build plugin
go build -o my-plugin.so -buildmode=plugin .

# Or build HashiCorp plugin
go build -o my-plugin ./main.go
```

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Run the test suite: `make test`
5. Commit your changes: `git commit -m 'Add amazing feature'`
6. Push to the branch: `git push origin feature/amazing-feature`
7. Open a Pull Request

## Community & Support

- **Discord** - [Discord](https://discord.gg/Xb7p2ddC6E)
- **GitHub Issues** - [GitHub Issues](https://github.com/apito-io/engine/issues)
- **Documentation** - [apito.io/docs](https://apito.io/docs)
- **Security** - Report security issues to [security@apito.io](mailto:security@apito.io)

## Documentation

For documentation, visit [apito.io/docs](https://apito.io/docs).

- [Quick start](https://apito.io/docs/quick-start/)
- [Plugins](https://apito.io/docs/developing-plugins/first-plugin/)
- [Databases](https://apito.io/docs/platform/database-design/)
- [Authentication](https://apito.io/docs/platform/auth/)
- [GraphQL](https://apito.io/docs)
- [Functions](https://apito.io/docs/platform/functions/)

## License

Licensed under AGPL-3.0. You can use, modify and self-host it. If you run a modified version as a network service, you must publish your changes under AGPL-3.0. A commercial license is available from Apito LLC. Releases v1.8.26 to v1.8.30 remain FSL-1.1-ALv2; see LICENSING.md.

## Badges

**Built with Apito**

[![Built with Apito](https://img.shields.io/badge/Built%20with-Apito-FF4081?style=flat&logo=graphql)](https://apito.io)

```markdown
[![Built with Apito](https://img.shields.io/badge/Built%20with-Apito-FF4081?style=flat&logo=graphql)](https://apito.io)
```

**Powered by Apito**

[![Powered by Apito](https://img.shields.io/badge/Powered%20by-Apito-FF4081?style=for-the-badge&logo=graphql)](https://apito.io)

```markdown
[![Powered by Apito](https://img.shields.io/badge/Powered%20by-Apito-FF4081?style=for-the-badge&logo=graphql)](https://apito.io)
```

---

<p align="center">
  Made with ❤️ by the Apito team and contributors
</p>

<p align="center">
  <a href="https://apito.io">Website</a> ·
  <a href="https://www.linkedin.com/company/apito-io">LinkedIn</a> ·
  <a href="https://github.com/apito-io">GitHub</a>
</p>
