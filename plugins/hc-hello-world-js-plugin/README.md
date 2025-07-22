# Hello World JavaScript HashiCorp Plugin

This is a JavaScript implementation of a HashiCorp go-plugin that demonstrates how to create plugins for the Apito Engine using Node.js instead of Go. This plugin implements the same functionality as the Go-based `hc-hello-world-plugin` but runs as a JavaScript process.

## 🎯 Overview

This plugin demonstrates:

- **Cross-Language Plugin Support**: JavaScript plugins working with HashiCorp's go-plugin framework
- **gRPC Communication**: Using gRPC for communication between Go host and JavaScript plugin
- **GraphQL Schema Registration**: Dynamic schema registration with complex object types
- **REST API Registration**: Custom REST endpoints from JavaScript
- **Function Execution**: Resolver and function execution in JavaScript
- **Protobuf Integration**: Full protobuf support for message serialization

## 📋 Features

### GraphQL Resolvers

- `JS_helloWorldQuery`: Basic hello world with object and array support (JS version)
- `JS_processComplexData`: Complex data processing with multiple input types (JS version)
- `JS_sayHelloMutation`: Custom message echoing (JS version)

### REST API Endpoints

- `GET /js-hello`: Simple hello endpoint
- `POST /js-hello`: Hello endpoint with POST data processing

### Plugin Capabilities

- Environment variable handling
- Migration support (no-op)
- Version reporting
- **gRPC Health Checking**: Implements standard gRPC health service required by go-plugin
- Graceful shutdown handling
- Error handling and logging
- **Host Registry Integration**: Registered in hashicorp_plugin_list.go for discovery

## 🚀 Prerequisites

- **Node.js**: Version 18.0.0 or higher
- **npm**: For dependency management
- **Apito Engine**: The host Go application

## 📦 Installation

1. **Install Dependencies**:

   ```bash
   cd hc-hello-world-js-plugin
   npm install
   ```

2. **Or use Make**:
   ```bash
   make install
   ```

## 🔧 Usage

### Running the Plugin

The plugin is designed to be launched by the HashiCorp go-plugin host system. However, you can test it manually:

```bash
# Set required environment variables
export APITO_PLUGIN=apito_plugin_magic_cookie_v1

# Run the plugin
npm start
# or
node index.js
# or
make run
```

### Integration with Go Host

The Go host should launch this plugin as follows:

```go
cmd := exec.Command("node", "/path/to/hc-hello-world-js-plugin/index.js")
cmd.Env = append(os.Environ(), "APITO_PLUGIN=apito_plugin_magic_cookie_v1")

client := plugin.NewClient(&plugin.ClientConfig{
    HandshakeConfig: handshakeConfig,
    Plugins:         pluginMap,
    Cmd:             cmd,
    AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
})
```

## 🏗️ Architecture

### Plugin Structure

```
hc-hello-world-js-plugin/
├── index.js          # Main plugin implementation
├── package.json      # Node.js dependencies and scripts
├── plugin.proto      # Protobuf service definitions
├── health.proto      # gRPC health check service definitions
├── test-plugin.js    # Local testing script
├── Makefile         # Build and run targets
└── README.md        # This file
```

### Class Structure

- **HelloWorldJSPlugin**: Main plugin class implementing all service methods
- **HealthService**: gRPC health check service implementation (required by go-plugin)
- **Service Methods**: `Init`, `Migration`, `SchemaRegister`, `RESTApiRegister`, `GetVersion`, `Execute`
- **Health Methods**: `Check`, `Watch` for health monitoring
- **Execution Handlers**: GraphQL, function, and REST execution handlers
- **Helper Methods**: Protobuf struct conversion utilities

## 🔌 Plugin Interface

### gRPC Service Implementation

The plugin implements the `apito.plugin.v1.PluginService` interface:

```protobuf
service PluginService {
  rpc Init(InitRequest) returns (InitResponse);
  rpc Migration(MigrationRequest) returns (MigrationResponse);
  rpc SchemaRegister(SchemaRegisterRequest) returns (SchemaRegisterResponse);
  rpc RESTApiRegister(RESTApiRegisterRequest) returns (RESTApiRegisterResponse);
  rpc GetVersion(GetVersionRequest) returns (GetVersionResponse);
  rpc Execute(ExecuteRequest) returns (ExecuteResponse);
}
```

### Magic Cookie Validation

The plugin validates the magic cookie to ensure it's launched by the correct host:

- **Key**: `APITO_PLUGIN`
- **Value**: `apito_plugin_magic_cookie_v1`
- **Protocol Version**: `1`

## 🧪 Testing

### GraphQL Queries

```graphql
# Simple hello query
query {
  helloWorldQuery(name: "JavaScript")
}

# Complex data processing
query {
  processComplexData(
    user: {
      id: 1
      name: "John"
      email: "john@example.com"
      age: 30
      active: true
    }
    tags: ["javascript", "nodejs", "grpc"]
    numbers: [1, 2, 3, 4, 5]
    users: [
      { id: 1, name: "Alice", email: "alice@example.com" }
      { id: 2, name: "Bob", email: "bob@example.com" }
    ]
  )
}
```

### GraphQL Mutations

```graphql
mutation {
  sayHelloMutation(message: "Hello from JavaScript!")
}
```

### REST API Testing

```bash
# GET request
curl http://localhost:8080/js-hello

# POST request
curl -X POST http://localhost:8080/js-hello \
  -H "Content-Type: application/json" \
  -d '{"name": "JavaScript User"}'
```

## 🛠️ Development

### Available Make Targets

```bash
make help           # Show available targets
make install        # Install dependencies
make start          # Start the plugin
make run            # Run plugin directly
make test           # Test the plugin
make lint           # Check syntax
make clean          # Clean dependencies
make update-deps    # Update dependencies
```

### Key Implementation Details

1. **Protobuf Conversion**: Custom helpers to convert between protobuf Struct/Value and JavaScript objects
2. **Error Handling**: Comprehensive error handling with proper gRPC status codes
3. **Logging**: Structured logging for debugging and monitoring
4. **Graceful Shutdown**: Proper SIGTERM/SIGINT handling
5. **Port Communication**: Outputs port information for go-plugin handshake

## 🔍 Comparison with Go Plugin

| Feature       | Go Plugin      | JavaScript Plugin                 |
| ------------- | -------------- | --------------------------------- |
| Language      | Go             | JavaScript/Node.js                |
| Binary Size   | ~16MB compiled | ~2MB + node_modules               |
| Startup Time  | Fast           | Slightly slower (Node.js startup) |
| Memory Usage  | Lower          | Higher (V8 overhead)              |
| Development   | Compiled       | Interpreted                       |
| Debugging     | Go tooling     | Node.js tooling                   |
| Functionality | ✅ Complete    | ✅ Complete                       |

## 🐛 Troubleshooting

### Common Issues

1. **Magic Cookie Error**: Ensure `APITO_PLUGIN` environment variable is set correctly
2. **Proto File Not Found**: Verify plugin.proto is in the correct location
3. **Port Binding Issues**: Check for port conflicts or firewall restrictions
4. **Node.js Version**: Ensure Node.js 18+ is installed

### Debug Mode

Set debug logging:

```bash
export DEBUG=*
node index.js
```

## 📚 References

- [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin)
- [gRPC Node.js](https://grpc.io/docs/languages/node/)
- [Protocol Buffers](https://developers.google.com/protocol-buffers)
- [Apito Engine Documentation](https://apito.io/docs)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## 📄 License

This plugin is part of the Apito Engine project and follows the same licensing terms.

---

**Note**: This demonstrates that HashiCorp's go-plugin framework can work with any language that supports gRPC, not just Go. The plugin communicates over gRPC, making it language-agnostic at the protocol level.
