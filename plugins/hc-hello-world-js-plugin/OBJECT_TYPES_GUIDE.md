# Dynamic Object Types in JavaScript Plugin GraphQL Schema

This guide demonstrates the new support for dynamic `Object` and `[Object]` types in the JavaScript plugin system. Unlike the previous hardcoded `UserInput` type, the new system allows plugins to define any object structure using the `properties` field, implemented in JavaScript/Node.js.

## 🎯 Key Features

- **Dynamic Object Definition**: Define object structures using `properties` field in JavaScript
- **Array Support**: Support for arrays of objects `[Object]`, `[Object]!`, `[Object!]`, `[Object!]!`
- **Flexible Properties**: Each object can have different properties as needed
- **Type Safety**: Full GraphQL type validation and error handling
- **Automatic Naming**: Dynamic input type names generated based on properties
- **Cross-Language Compatibility**: JavaScript plugin works with Go host system

## 📋 Requirements

When using `Object` types in JavaScript plugin schemas:

1. **Properties Required**: Any argument with type `Object` or `[Object]` variants **MUST** include a `properties` field
2. **Properties Structure**: Properties must be a JavaScript object where keys are field names and values define the field type and description
3. **Validation**: Missing properties will cause an error and fall back to `String` type
4. **JavaScript Conversion**: Protobuf structs are automatically converted to JavaScript objects

## 🔧 JavaScript Plugin Schema Definition

### Single Object Input

```javascript
user: {
    type: 'Object',
    description: 'A single user object input',
    properties: {
        id: {
            type: 'Int',
            description: 'User ID'
        },
        name: {
            type: 'String',
            description: 'User name'
        },
        email: {
            type: 'String',
            description: 'User email'
        },
        age: {
            type: 'Int',
            description: 'User age'
        },
        active: {
            type: 'Boolean',
            description: 'Whether user is active'
        }
    }
}
```

### Array of Objects Input

```javascript
users: {
    type: '[Object!]',
    description: 'Array of required user objects',
    properties: {
        id: {
            type: 'Int',
            description: 'User ID'
        },
        name: {
            type: 'String',
            description: 'User name'
        },
        email: {
            type: 'String',
            description: 'User email'
        }
    }
}
```

### Simple Object Example

```javascript
product: {
    type: 'Object',
    description: 'Product information',
    properties: {
        name: {
            type: 'String',
            description: 'Product name'
        },
        price: {
            type: 'Float',
            description: 'Product price'
        },
        inStock: {
            type: 'Boolean',
            description: 'Whether product is in stock'
        }
    }
}
```

## 🔍 GraphQL Query Examples

### Query with Simple Object

```graphql
query {
  helloWorldQuery(name: "John", object: { name: "Test Object", age: 25 })
}
```

### Query with Array of Objects

```graphql
query {
  helloWorldQuery(
    name: "John"
    arrayofObjects: [
      { name: "Object 1", age: 30 }
      { name: "Object 2", age: 35 }
    ]
  )
}
```

### Complex Query with All Types

```graphql
query {
  processComplexData(
    user: {
      id: 1
      name: "John Doe"
      email: "john@example.com"
      age: 30
      active: true
    }
    tags: ["javascript", "nodejs", "grpc"]
    numbers: [10, 20, 30]
    users: [
      { id: 2, name: "Alice", email: "alice@example.com" }
      { id: 3, name: "Bob", email: "bob@example.com" }
    ]
    optionalUsers: [
      { name: "Optional User", email: "optional@example.com" }
      null
    ]
  )
}
```

## 🏗️ Implementation Details

### Host Engine Processing

The engine automatically:

1. **Detects Object Types**: Recognizes `Object`, `Object!`, `[Object]`, `[Object]!`, `[Object!]`, `[Object!]!`
2. **Validates Properties**: Ensures `properties` field exists for Object types
3. **Creates Dynamic Types**: Generates unique GraphQL input object types
4. **Names Types**: Uses field names to create consistent type names like `DynamicInput_name_age`
5. **Handles Arrays**: Properly wraps object types in GraphQL lists
6. **Cross-Language Support**: Works with both Go and JavaScript plugins

### JavaScript Plugin Processing

JavaScript plugins receive arguments as:

- **Objects**: JavaScript objects with field values
- **Arrays**: JavaScript arrays containing object instances
- **Type Conversion**: Numbers as `number`, strings as `string`, booleans as `boolean`
- **Protobuf Conversion**: Automatic conversion from protobuf Struct to JavaScript objects

### Example JavaScript Plugin Handler

```javascript
executeProcessComplexDataResolver(args) {
    // Handle single object
    if (args.user) {
        const user = args.user;
        console.log(`User: ${user.name}, Age: ${user.age}, Active: ${user.active}`);

        // JavaScript type checking
        if (typeof user.name === 'string') {
            // Process string
        }
        if (typeof user.age === 'number') {
            // Process number
        }
        if (typeof user.active === 'boolean') {
            // Process boolean
        }
    }

    // Handle array of objects
    if (args.users && Array.isArray(args.users)) {
        args.users.forEach((user, index) => {
            console.log(`User ${index + 1}: ${user.name} (${user.email})`);
        });
    }

    // Handle array of strings
    if (args.tags && Array.isArray(args.tags)) {
        console.log(`Tags: ${args.tags.join(', ')}`);
    }

    // Handle array of numbers
    if (args.numbers && Array.isArray(args.numbers)) {
        console.log(`Numbers: ${args.numbers.join(', ')}`);
    }

    return 'Processing complete from JavaScript plugin';
}
```

### Protobuf Struct Conversion

The JavaScript plugin includes helper methods for converting protobuf structures:

```javascript
// Convert protobuf Struct to JavaScript object
structToObject(struct) {
    if (!struct || !struct.fields) {
        return {};
    }

    const result = {};
    for (const [key, value] of Object.entries(struct.fields)) {
        result[key] = this.valueToJS(value);
    }
    return result;
}

// Convert protobuf Value to JavaScript value
valueToJS(value) {
    if (!value) return null;

    if (value.nullValue !== undefined) {
        return null;
    } else if (value.numberValue !== undefined) {
        return value.numberValue;
    } else if (value.stringValue !== undefined) {
        return value.stringValue;
    } else if (value.boolValue !== undefined) {
        return value.boolValue;
    } else if (value.structValue) {
        return this.structToObject(value.structValue);
    } else if (value.listValue) {
        return value.listValue.values.map(v => this.valueToJS(v));
    }

    return null;
}
```

## 🚀 JavaScript-Specific Features

### Native JavaScript Operations

```javascript
// Working with arrays
if (Array.isArray(args.users)) {
  const activeUsers = args.users.filter((user) => user.active);
  const userNames = args.users.map((user) => user.name);
  const userCount = args.users.length;
}

// Working with objects
const user = args.user;
const userKeys = Object.keys(user);
const hasEmail = "email" in user;
const userJson = JSON.stringify(user, null, 2);

// String manipulation
const greeting = `Hello, ${user.name}! You are ${user.age} years old.`;
const tags = args.tags.join(", ");

// Number operations
const sum = args.numbers.reduce((acc, num) => acc + num, 0);
const average = sum / args.numbers.length;
```

### Error Handling

```javascript
try {
  // Process complex data
  const result = this.processUserData(args.user);
  return result;
} catch (error) {
  console.error("Processing error:", error);
  throw new Error(`Processing failed: ${error.message}`);
}
```

### Async Operations

```javascript
async executeComplexOperation(args) {
    try {
        // Simulate async operation
        const result = await this.processDataAsync(args);
        return result;
    } catch (error) {
        throw new Error(`Async operation failed: ${error.message}`);
    }
}
```

## 🔄 Comparison: Go vs JavaScript

| Feature               | Go Plugin                | JavaScript Plugin         |
| --------------------- | ------------------------ | ------------------------- |
| **Object Handling**   | `map[string]interface{}` | Native JavaScript objects |
| **Type Assertions**   | `obj.(string)`           | `typeof obj === 'string'` |
| **Array Processing**  | `[]interface{}`          | Native JavaScript arrays  |
| **JSON Handling**     | `json.Marshal/Unmarshal` | `JSON.stringify/parse`    |
| **String Formatting** | `fmt.Sprintf`            | Template literals         |
| **Error Handling**    | `error` interface        | `Error` objects/try-catch |
| **Async Support**     | Goroutines               | Promises/async-await      |

## 🧪 Testing JavaScript Plugin

### Manual Testing

```bash
# Set environment variable
export APITO_PLUGIN=apito_plugin_magic_cookie_v1

# Run the plugin
node index.js
```

### Integration Testing

```javascript
// test-plugin.js
const { HelloWorldJSPlugin } = require("./index.js");

async function testPlugin() {
  const plugin = new HelloWorldJSPlugin();

  // Test complex data processing
  const args = {
    user: {
      id: 1,
      name: "Test User",
      email: "test@example.com",
      age: 25,
      active: true,
    },
    tags: ["javascript", "testing"],
    numbers: [1, 2, 3, 4, 5],
    users: [
      { id: 1, name: "User 1", email: "user1@example.com" },
      { id: 2, name: "User 2", email: "user2@example.com" },
    ],
  };

  const result = plugin.executeProcessComplexDataResolver(args);
  console.log(result);
}

testPlugin();
```

## 📚 Best Practices for JavaScript Plugins

1. **Type Validation**: Always validate input types before processing
2. **Error Handling**: Use try-catch blocks for robust error handling
3. **Async Operations**: Use async/await for better readability
4. **Memory Management**: Be mindful of memory usage with large objects
5. **Logging**: Use structured logging for debugging
6. **Performance**: Consider using streaming for large datasets

## 🔧 Debugging Tips

### Enable Debug Logging

```bash
export DEBUG=*
node index.js
```

### Use Node.js Inspector

```bash
node --inspect index.js
```

### Log Object Structure

```javascript
console.log("Args structure:", JSON.stringify(args, null, 2));
console.log("Object keys:", Object.keys(args));
console.log(
  "Array length:",
  Array.isArray(args.users) ? args.users.length : "Not an array"
);
```

## 🌟 Advantages of JavaScript Plugin

1. **Rapid Development**: No compilation step required
2. **Rich Ecosystem**: Access to npm packages
3. **Dynamic Typing**: Flexible object manipulation
4. **JSON Native**: Built-in JSON support
5. **Async Support**: First-class async/await support
6. **Debugging**: Excellent debugging tools

## 🚧 Limitations

1. **Performance**: Slower than compiled Go plugins
2. **Memory Usage**: Higher memory footprint
3. **Startup Time**: Node.js startup overhead
4. **Type Safety**: Runtime type checking vs compile-time

---

**Note**: This JavaScript implementation proves that HashiCorp's go-plugin architecture is truly language-agnostic, allowing developers to choose the best language for their specific plugin requirements while maintaining full compatibility with the Go host system.
