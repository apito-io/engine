# BBolt vs Mongo: Function-by-Function Comparison Checklist

## Helper Methods
- [x] `getCollectionName` - ✅ Matches Mongo pattern
- [x] `getRelationCollectionName` - ✅ Matches
- [x] `getMediaCollectionName` - ✅ Matches
- [ ] `getEnv` - ❌ MISSING in BBolt
- [x] `searchAndAppend` - ✅ NOW ADDED

## Core Interface Methods (47 total)

### Project Management
1. [ ] `DeleteProject` - ⚠️ WRONG logic - needs to match Mongo's collection dropping
2. [ ] `TransferProject` - ⚠️ WRONG - needs transferDocs, transferMediaDocs, transferRelationDocs helpers

### Collection Management  
3. [ ] `CheckCollectionExists` - ⚠️ WRONG - needs to match Mongo's naming (regular vs SaaS)
4. [x] `AddCollection` - ✅ Currently no-op (need to verify if correct)

### Model Management
5. [x] `AddModel` - ✅ NOW CORRECT (matches Mongo)
6. [x] `AddFieldToModel` - ✅ NOW CORRECT (matches Mongo with searchAndAppend)
7. [x] `RenameModel` - ✅ NOW CORRECT (matches Mongo)
8. [x] `ConvertModel` - ✅ No-op (correct)
9. [x] `DropModel` - ✅ NOW CORRECT (matches Mongo)

### Index Management
10. [ ] `CreateIndex` - ⚠️ Needs to match Mongo's logic
11. [x] `DropIndex` - ✅ No-op (correct for ApitoBolt)

### Document Operations
12. [ ] `GetSingleProjectDocument` - ⚠️ Need to verify collection naming matches Mongo
13. [x] `GetSingleProjectDocumentBytes` - ✅ Wrapper (correct)
14. [ ] `GetSingleProjectDocumentRevisions` - ⚠️ Need revision collection logic
15. [x] `GetSingleRawDocumentFromProject` - ✅ Wrapper (correct)
16. [ ] `QueryMultiDocumentOfProject` - ⚠️ WRONG - needs Mongo's filter logic
17. [x] `QueryMultiDocumentOfProjectBytes` - ✅ Wrapper (correct)
18. [ ] `AddDocumentToProject` - ⚠️ Need to verify Mongo logic matches
19. [ ] `UpdateDocumentOfProject` - ⚠️ Need to verify replace vs update logic
20. [ ] `DeleteDocumentFromProject` - ⚠️ Need collection name logic
21. [ ] `DeleteDocumentsFromProject` - ⚠️ Need filter logic
22. [ ] `DeleteDocumentRelation` - ⚠️ Need relation collection logic

### Counting
23. [x] `CountDocOfProject` - ✅ Wrapper (correct)
24. [x] `CountDocOfProjectBytes` - ✅ Wrapper (correct)
25. [ ] `CountMultiDocumentOfProject` - ⚠️ Need filter logic

### Field Operations
26. [x] `DropField` - ✅ NOW HAS tenant filtering (verify collection naming)
27. [x] `RenameField` - ✅ NOW HAS tenant filtering (verify collection naming)

### Relation Operations
28. [x] `AddRelationFields` - ✅ No-op (correct)
29. [ ] `DeleteRelationDocuments` - ⚠️ Need to verify
30. [ ] `GetRelationDocument` - ⚠️ Need to verify
31. [ ] `CreateRelation` - ⚠️ Need to verify
32. [ ] `DeleteRelation` - ⚠️ Need to verify
33. [ ] `NewInsertableRelations` - ⚠️ WRONG - stub implementation
34. [ ] `CheckOneToOneRelationExists` - ⚠️ Need to verify
35. [ ] `GetRelationIds` - ⚠️ Need to verify

### Builder Operations
36. [ ] `ConnectBuilder` - ⚠️ WRONG - stub, need Mongo logic
37. [ ] `DisconnectBuilder` - ⚠️ WRONG - stub, need Mongo logic

### User Operations
38. [ ] `GetProjectUser` - ⚠️ WRONG - stub, need Mongo logic
39. [ ] `GetLoggedInProjectUser` - ⚠️ WRONG - need Mongo logic
40. [ ] `GetProjectUsers` - ⚠️ WRONG - stub, need Mongo logic
41. [ ] `GetAllRelationDocumentsOfSingleDocument` - ⚠️ Need Mongo logic

### Metadata
42. [x] `AddTeamMetaInfo` - ⚠️ Need Mongo logic

### Data Loading
43. [ ] `RelationshipDataLoader` - ⚠️ WRONG - stub, need Mongo logic
44. [x] `RelationshipDataLoaderBytes` - ✅ Wrapper (correct)

### Aggregation
45. [ ] `AggregateDocOfProject` - ⚠️ WRONG - stub, need Mongo logic
46. [x] `AggregateDocOfProjectBytes` - ✅ Wrapper (correct)

### Extra Mongo Methods (not in BBolt yet)
47. [ ] `SearchFunctions` - ❌ MISSING
48. [ ] `SearchWebHooks` - ❌ MISSING
49. [ ] `GetWebHook` - ❌ MISSING
50. [ ] `GetProject` - ❌ MISSING

## CRITICAL Issues Found

### 1. Collection Naming (MOST IMPORTANT)
Mongo has TWO patterns:
- **SaaS**: `p_{projectId}_{modelName}`
- **Regular**: Just `{modelName}` (NOT `p_{projectId}_{modelName}`)

BBolt currently always uses: `p_{projectId}_{modelName}`

### 2. Missing Helper Functions
- `transferDocs()`
- `transferMediaDocs()`  
- `transferRelationDocs()`
- `getEnv()`

### 3. Wrong Stubs
Many functions are stubs that return errors or empty data instead of real implementations.

## Action Plan

1. Fix `CheckCollectionExists` - use correct naming (SaaS vs Regular)
2. Fix `getCollectionName` - handle Regular vs SaaS differently
3. Add missing helper functions
4. Implement all stub functions with Mongo logic
5. Verify every function matches Mongo's behavior

