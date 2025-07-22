# Memory KV Driver

A thread-safe in-memory key-value storage driver that implements the `KeyValueServiceInterface` for local development and testing.

## Features

- **Thread-safe**: Uses `sync.RWMutex` for concurrent access
- **Expiration support**: Automatic cleanup of expired keys
- **Multiple data structures**:
  - Key-value pairs with optional TTL
  - Hash maps
  - Sets
  - Sorted sets
- **JSON support**: Built-in JSON serialization/deserialization
- **Memory efficient**: Automatic cleanup of expired items and empty collections

## Usage

```go
import (
    kvMemory "github.com/apito-io/engine/database/kv/memory"
    "github.com/apito-io/engine/models"
)

// Create a new memory KV service
cfg := &models.Config{}
kvService, err := kvMemory.GetKVMemoryDriver(cfg)
if err != nil {
    log.Fatal(err)
}
defer kvService.Close()

// Use the service
ctx := context.Background()
err = kvService.SetValue(ctx, "key", "value", time.Hour)
```

## Configuration

Set the KV storage engine to memory in your environment:

```bash
export KV_ENGINE=memory
```

Or in your configuration:

```go
cfg.KVStorageEngine = _const.MemoryDb
```

## Methods Implemented

- `SetValue` / `GetValue` - Basic key-value operations
- `SetJSONObject` / `GetJSONObject` - JSON object storage
- `SetToHashMap` / `GetFromHashMap` / `CheckKeyHashMap` - Hash map operations
- `AddToSets` / `RemoveSets` / `GetStoreDomains` - Set operations
- `AddToSortedSets` / `GetFromSortedSets` - Sorted set operations
- `DelValue` - Delete keys
- `CheckRedisKey` - Check key existence

## Automatic Cleanup

The driver automatically:

- Removes expired keys every minute
- Cleans up empty collections
- Provides thread-safe access to all operations

## Testing

Run the test suite:

```bash
go test -v
```

## Use Cases

- Local development without external dependencies
- Unit testing
- Lightweight caching for single-instance applications
- Development environments where Redis/Badger is not available
