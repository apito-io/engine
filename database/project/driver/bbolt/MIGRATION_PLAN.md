# BBolt Driver Rewrite: Mongo-Style Implementation Plan

## Key Differences: Mongo vs Current BBolt

### Structure
| Aspect | Mongo | Current BBolt | Needs Change |
|--------|-------|---------------|--------------|
| File Structure | init.go + implementation.go | init.go + functions.go + misc.go | ✅ YES |
| Total Lines | 57 + 1860 = 1917 | 362 + 645 + 499 = 1506 | Consolidate |
| Organization | All logic in implementation.go | Split across 3 files | Merge to implementation.go |

### Collection Strategy
| Aspect | Mongo | Current BBolt | Needs Change |
|--------|-------|---------------|--------------|
| **Document Storage** | Each model = separate collection<br>`p_{projectId}_{modelName}` | Single "project_docs" collection<br>Manual filtering by projectId + modelName | ✅ **CRITICAL** |
| **Relations** | `p_{projectId}_relation` collection | Single "project_relations" collection | ✅ YES |
| **Media** | `p_{projectId}_media` | Not implemented | Add later |
| **Revisions** | Per-model revision collections | Single "project_revisions" | ✅ YES |

### Naming Patterns
```go
// Mongo Pattern
Documents:  "p_{projectId}_{modelName}"          // e.g., p_abc123_users
Relations:  "p_{projectId}_relation"             // e.g., p_abc123_relation  
Media:      "p_{projectId}_media"                // e.g., p_abc123_media

// Current BBolt (WRONG)
Documents:  "project_docs" + filter by projectId + modelName
Relations:  "project_relations" + filter
```

### Index Creation
| Aspect | Mongo | Current BBolt | Needs Change |
|--------|-------|---------------|--------------|
| TTL Index | `expire_at` with expireAfterSeconds | Not implemented | ✅ Add |
| Type Index | `type` field | Not implemented | ✅ Add |
| Status Index | `meta.status` | Not implemented | ✅ Add |
| Relation Indexes | `from`, `to`, `from_id`, `to_id`, `known_as` | Basic ApitoBolt indexes | ✅ Enhance |
| Tenant Index (SaaS) | `tenant_id` | Not implemented | ✅ Add |

### Data Access Patterns
```go
// Mongo - Direct collection access
collection := m.Database.Collection(collectionName)
cursor, err := collection.Find(ctx, filter, findOpts)

// Current BBolt - Manual filtering (INEFFICIENT)
docsCol := tx.Collection("project_docs")
docsCol.All(&docs)  // Get ALL docs
// Then filter: if doc.ProjectID == projectID && doc.ModelName == modelName

// Should Be - ApitoBolt collection-per-model
modelCollection := b.Store.Collection(fmt.Sprintf("p_%s_%s", projectID, modelName))
// Now queries are scoped to this collection automatically!
```

## Migration Checklist

### Phase 1: Restructure Files ✅
- [ ] Create new `implementation.go`
- [ ] Move all methods from `functions.go` to `implementation.go`
- [ ] Move all methods from `misc.go` to `implementation.go`
- [ ] Keep `init.go` minimal (just connection + struct)
- [ ] Delete old `functions.go` and `misc.go`

### Phase 2: Fix Collection Naming Strategy ✅ CRITICAL
- [ ] Implement `getCollectionName()` helper like Mongo
- [ ] Change from generic collections to per-model collections:
  - `"project_docs"` → `"p_{projectId}_{modelName}"`
  - `"project_relations"` → `"p_{projectId}_relation"`
  - `"project_revisions"` → `"p_{projectId}_{modelName}_revisions"`
- [ ] Update all collection access to use new naming
- [ ] Remove manual projectId+modelName filtering (now automatic)

### Phase 3: Enhance Index Creation ✅
- [ ] Add TTL indexes for `expire_at`
- [ ] Add `type` field indexes
- [ ] Add `meta.status` indexes
- [ ] Add proper relation indexes (`from`, `to`, `from_id`, `to_id`, `known_as`)
- [ ] Add SaaS `tenant_id` indexes

### Phase 4: Update Document Operations ✅
- [ ] Use collection-scoped queries instead of filtering
- [ ] Implement proper `Find()` with bson-like filters
- [ ] Use `InsertOne` pattern instead of `Save` where appropriate
- [ ] Add proper query options (limit, skip, sort)

### Phase 5: Add Missing Features
- [ ] Implement media collection support
- [ ] Add transaction support (if ApitoBolt supports it)
- [ ] Add helper methods: `getCollectionName()`, `getRelationCollection()`
- [ ] Add proper error handling like Mongo

### Phase 6: Test & Verify
- [ ] Test document CRUD with new collection strategy
- [ ] Test relations with scoped collections
- [ ] Test index creation
- [ ] Verify no performance regression
- [ ] Compare with Mongo driver behavior

## Code Examples

### Before (Current - WRONG)
```go
func (b *BBoltDriver) AddDocumentToProject(...) {
    // Stores in generic "project_docs" collection
    docsCol := tx.Collection("project_docs")
    projectDoc := ProjectDocument{
        ID:        docKey,
        ProjectID: param.ProjectID,  // Manual tracking
        ModelName: param.Model.Name, // Manual tracking
        Data:      doc.Data,
    }
    docsCol.Save(&projectDoc)
}

func (b *BBoltDriver) QueryMultiDocumentOfProject(...) {
    docsCol.All(&projectDocs)  // Gets ALL documents!
    for _, doc := range projectDocs {
        // Manual filtering - SLOW!
        if doc.ProjectID == param.ProjectID && doc.ModelName == param.Model.Name {
            result = append(result, doc)
        }
    }
}
```

### After (Mongo-Style - CORRECT)
```go
func (b *BBoltDriver) AddDocumentToProject(...) {
    // Each model has its own collection!
    collectionName := fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
    modelCol := b.Store.Collection(collectionName)
    
    // No need to track projectID/modelName - it's in the collection name!
    doc := types.DefaultDocumentStructure{
        ID:   docID,
        Data: data,
        Meta: &types.MetaField{...},
    }
    modelCol.Save(&doc)  // Automatically scoped to this model
}

func (b *BBoltDriver) QueryMultiDocumentOfProject(...) {
    // Direct collection access - only this model's docs!
    collectionName := fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
    modelCol := b.Store.Collection(collectionName)
    
    var results []*types.DefaultDocumentStructure
    modelCol.All(&results)  // Already filtered by collection!
    // No manual filtering needed!
}
```

## Benefits of Mongo-Style Approach

1. **Performance**: No need to scan all documents - collection isolation
2. **Scalability**: Each model grows independently
3. **Simplicity**: No manual projectID/modelName filtering
4. **Indexes**: Per-collection indexes are more efficient
5. **Isolation**: Projects don't interfere with each other
6. **Familiar**: Matches Mongo patterns developers know

## Implementation Order

1. ✅ Phase 1: Restructure (low risk)
2. ✅ Phase 2: Collection naming (CRITICAL - breaks compatibility but correct)
3. ✅ Phase 3: Indexes (enhancement)
4. ✅ Phase 4: Operations (use new structure)
5. ✅ Phase 5: Features (additions)
6. ✅ Phase 6: Test (verification)

## Notes

- ApitoBolt is Mongo-like, so this approach is NATURAL
- Current approach treats it like key-value store (wrong abstraction)
- Mongo driver has 7+ years of production battle-testing
- Following Mongo patterns = following best practices

