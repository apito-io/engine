# Dynamic Object Types in Plugin GraphQL Schema

This guide demonstrates the new support for dynamic `Object` and `[Object]` types in the plugin system. Unlike the previous hardcoded `UserInput` type, the new system allows plugins to define any object structure using the `properties` field.

## 🎯 Key Features

- **Dynamic Object Definition**: Define object structures using `properties` field
- **Array Support**: Support for arrays of objects `[Object]`, `[Object]!`, `[Object!]`, `[Object!]!`
- **Flexible Properties**: Each object can have different properties as needed
- **Type Safety**: Full GraphQL type validation and error handling
- **Automatic Naming**: Dynamic input type names generated based on properties

## 📋 Requirements

When using `Object` types in plugin schemas:

1. **Properties Required**: Any argument with type `Object` or `[Object]` variants **MUST** include a `properties` field
2. **Properties Structure**: Properties must be a map where keys are field names and values define the field type and description
3. **Validation**: Missing properties will cause an error and fall back to `String` type

## 🔧 Plugin Schema Definition

### Single Object Input

```go
"user": map[string]interface{}{
    "type":        "Object",
    "description": "A single user object input",
    "properties": map[string]interface{}{
        "id": map[string]interface{}{
            "type":        "Int",
            "description": "User ID",
        },
        "name": map[string]interface{}{
            "type":        "String",
            "description": "User name",
        },
        "email": map[string]interface{}{
            "type":        "String",
            "description": "User email",
        },
        "age": map[string]interface{}{
            "type":        "Int",
            "description": "User age",
        },
        "active": map[string]interface{}{
            "type":        "Boolean",
            "description": "Whether user is active",
        },
    },
},
```

### Array of Objects Input

```go
"users": map[string]interface{}{
    "type":        "[Object!]",
    "description": "Array of required user objects",
    "properties": map[string]interface{}{
        "id": map[string]interface{}{
            "type":        "Int",
            "description": "User ID",
        },
        "name": map[string]interface{}{
            "type":        "String",
            "description": "User name",
        },
        "email": map[string]interface{}{
            "type":        "String",
            "description": "User email",
        },
    },
},
```

### Simple Object Example

```go
"product": map[string]interface{}{
    "type":        "Object",
    "description": "Product information",
    "properties": map[string]interface{}{
        "name": map[string]interface{}{
            "type":        "String",
            "description": "Product name",
        },
        "price": map[string]interface{}{
            "type":        "Float",
            "description": "Product price",
        },
        "inStock": map[string]interface{}{
            "type":        "Boolean",
            "description": "Whether product is in stock",
        },
    },
},
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
    tags: ["developer", "javascript", "react"]
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

### Plugin Processing

Plugins receive arguments as:

- **Objects**: `map[string]interface{}` with field values
- **Arrays**: `[]interface{}` containing object maps
- **Type Conversion**: Numbers come as `float64`, strings as `string`, booleans as `bool`

### Example Plugin Handler

```go
func (p *Plugin) handleObjectInput(args map[string]interface{}) (interface{}, error) {
    // Handle single object
    if obj, exists := args["user"]; exists && obj != nil {
        if userMap, ok := obj.(map[string]interface{}); ok {
            if name, ok := userMap["name"].(string); ok {
                // Process name
            }
            if age, ok := userMap["age"].(float64); ok {
                // Process age (convert to int if needed)
                ageInt := int(age)
            }
        }
    }

    // Handle array of objects
    if users, exists := args["users"]; exists && users != nil {
        if userSlice, ok := users.([]interface{}); ok {
            for _, user := range userSlice {
                if userMap, ok := user.(map[string]interface{}); ok {
                    // Process each user object
                }
            }
        }
    }

    return "Processing complete", nil
}
```

## 📊 Supported Type Mappings

| Plugin Schema Type | GraphQL Type                          | Description                        |
| ------------------ | ------------------------------------- | ---------------------------------- |
| `Object`           | `InputObject`                         | Optional object input              |
| `Object!`          | `NonNull(InputObject)`                | Required object input              |
| `[Object]`         | `List(InputObject)`                   | Optional array of optional objects |
| `[Object]!`        | `NonNull(List(InputObject))`          | Required array of optional objects |
| `[Object!]`        | `List(NonNull(InputObject))`          | Optional array of required objects |
| `[Object!]!`       | `NonNull(List(NonNull(InputObject)))` | Required array of required objects |

## 🎯 Real-World Examples

### E-commerce Plugin

```go
"createOrder": map[string]interface{}{
    "type": "String",
    "description": "Create a new order",
    "args": map[string]interface{}{
        "customer": map[string]interface{}{
            "type": "Object!",
            "description": "Customer information",
            "properties": map[string]interface{}{
                "name": map[string]interface{}{"type": "String!", "description": "Customer name"},
                "email": map[string]interface{}{"type": "String!", "description": "Customer email"},
                "phone": map[string]interface{}{"type": "String", "description": "Customer phone"},
            },
        },
        "items": map[string]interface{}{
            "type": "[Object!]!",
            "description": "Order items",
            "properties": map[string]interface{}{
                "productId": map[string]interface{}{"type": "ID!", "description": "Product ID"},
                "quantity": map[string]interface{}{"type": "Int!", "description": "Quantity"},
                "price": map[string]interface{}{"type": "Float!", "description": "Unit price"},
            },
        },
        "shippingAddress": map[string]interface{}{
            "type": "Object",
            "description": "Shipping address (optional)",
            "properties": map[string]interface{}{
                "street": map[string]interface{}{"type": "String!", "description": "Street address"},
                "city": map[string]interface{}{"type": "String!", "description": "City"},
                "zipCode": map[string]interface{}{"type": "String!", "description": "ZIP code"},
                "country": map[string]interface{}{"type": "String!", "description": "Country"},
            },
        },
    },
},
```

### User Management Plugin

```go
"updateUsers": map[string]interface{}{
    "type": "String",
    "description": "Batch update users",
    "args": map[string]interface{}{
        "updates": map[string]interface{}{
            "type": "[Object!]!",
            "description": "User updates",
            "properties": map[string]interface{}{
                "userId": map[string]interface{}{"type": "ID!", "description": "User ID to update"},
                "changes": map[string]interface{}{"type": "Object!", "description": "Changes to apply",
                    "properties": map[string]interface{}{
                        "name": map[string]interface{}{"type": "String", "description": "New name"},
                        "email": map[string]interface{}{"type": "String", "description": "New email"},
                        "role": map[string]interface{}{"type": "String", "description": "New role"},
                        "active": map[string]interface{}{"type": "Boolean", "description": "Active status"},
                    },
                },
            },
        },
    },
},
```

## ✅ Best Practices

1. **Always Define Properties**: Never use `Object` types without `properties`
2. **Use Descriptive Names**: Make property names clear and consistent
3. **Handle Null Values**: Check for null values in optional arrays
4. **Type Conversion**: Remember that JSON numbers become `float64` in Go
5. **Validation**: Validate object structures in your plugin handlers
6. **Error Handling**: Provide meaningful error messages for invalid inputs

## 🚀 Migration from UserInput

If you were using the old `UserInput` type, migrate like this:

**Old (deprecated):**

```go
"user": map[string]interface{}{
    "type": "UserInput",
    "description": "User input",
},
```

**New (recommended):**

```go
"user": map[string]interface{}{
    "type": "Object",
    "description": "User input",
    "properties": map[string]interface{}{
        "id": map[string]interface{}{"type": "Int", "description": "User ID"},
        "name": map[string]interface{}{"type": "String", "description": "User name"},
        "email": map[string]interface{}{"type": "String", "description": "User email"},
        "age": map[string]interface{}{"type": "Int", "description": "User age"},
        "active": map[string]interface{}{"type": "Boolean", "description": "Active status"},
    },
},
```

This new system provides much more flexibility and allows each plugin to define exactly the object structures it needs!
