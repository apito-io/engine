# GraphQL model naming matrix

Authoritative reference for **how stored model ids** (`definedModel.Name`) map to **GraphQL field names**, **mutation registration keys**, and **generated type names** in the public schema. This complements canonical naming rules in `open-core/utility/apito_naming.go` and the schema builder in `open-core/controller/public_schema_builder_build.go`.

## Stored model id

Throughout this document, **`stored`** means the string persisted on the model (canonical `snake_case` such as `food_order`, or legacy `camelCase` such as `foodCategory` where still present). Helpers use **`PascalFromAnyModelID`**, **`CamelFromAny`**, and **`GraphQLComposedTypeName`** so both shapes are supported; new projects should use **canonical snake_case**.

## Root query fields (registration keys)

| Artifact              | Formula / source                                                   | Example (`stored` = `food_order`) |
| --------------------- | ------------------------------------------------------------------ | --------------------------------- |
| Single document field | `SingularResourceName(stored)` → `CamelFromAny`                    | `foodOrder`                       |
| List field            | `MultipleResourceName(stored)` → `SingularResourceName` + `"List"` | `foodOrderList`                   |
| Count field           | `MultipleResourceName(stored) + "Count"`                           | `foodOrderListCount`              |

Implementation: `open-core/utility/name_extractor.go` (`SingularResourceName`, `MultipleResourceName`).

## Root mutation field names (registration keys)

Keys are **lowerCamel action + Pascal model** (no underscore between action and model).

| Mutation    | Map key                                   | Example (`food_order`) |
| ----------- | ----------------------------------------- | ---------------------- |
| Create one  | `"create" + PascalFromAnyModelID(stored)` | `createFoodOrder`      |
| Upsert many | `"upsert" + ListGraphQLTypeName(stored)`  | `upsertFoodOrderList`  |
| Update      | `"update" + PascalFromAnyModelID(stored)` | `updateFoodOrder`      |
| Delete      | `"delete" + PascalFromAnyModelID(stored)` | `deleteFoodOrder`      |

## GraphQL **object** and **input** type names (Pascal / composed)

| Role                                               | Source                                                                             | Example (`food_order`)           |
| -------------------------------------------------- | ---------------------------------------------------------------------------------- | -------------------------------- |
| Single row object                                  | `PascalFromAnyModelID(stored)`                                                     | `FoodOrder`                      |
| List row object (element type of list query)       | `ListGraphQLTypeName(stored)` = `PascalFromAnyModelID(stored) + "List"`            | `FoodOrderList`                  |
| Count connection wrapper                           | `GraphQLComposedTypeName(stored, "List_Connection")`                               | `Food_Order_List_Connection`     |
| Count / aggregate (when present)                   | `GraphQLComposedTypeName(stored, "List_Aggregate")`, `"...List_Aggregate_GroupBy"` | `Food_Order_List_Aggregate`, …   |
| Create result type                                 | `"Create_" + PascalFromAnyModelID(stored)`                                         | `Create_FoodOrder`               |
| Update result type                                 | `"Update_" + PascalFromAnyModelID(stored)`                                         | `Update_FoodOrder`               |
| Delete result type                                 | `"Delete_" + PascalFromAnyModelID(stored)`                                         | `Delete_FoodOrder`               |
| Upsert list result element                         | `"Upsert_" + ListGraphQLTypeName(stored)`                                          | `Upsert_FoodOrderList`           |
| Create payload input                               | `GraphQLComposedTypeName(stored, "Create_Payload")`                                | `Food_Order_Create_Payload`      |
| Update payload input                               | `GraphQLComposedTypeName(stored, "Update_Payload")`                                | `Food_Order_Update_Payload`      |
| Plural upsert payload item                         | `GraphQLComposedTypeName(stored, "List_Upsert_Payload")`                           | `Food_Order_List_Upsert_Payload` |
| List connect fragment (when model has connections) | `GraphQLComposedTypeName(stored, "List_Connect")`                                  | `Food_Order_List_Connect`        |

`GraphQLComposedTypeName` builds **underscore-separated** Title_Segments from the model id segments plus suffix segments (`open-core/utility/apito_naming.go`).

## Filter / sort arguments: fixed field names, model-specific **types**

`BuildFilterArgument` in `open-core/schemas/objects/search_filter_arg.go` always exposes the same **argument names** on list and count fields:

`page`, `limit`, `local`, `status`, `_key`, `connection` (if any), `groupBy`, `relation` (if any), `where` (if any), `sort` (if any).

Only the **GraphQL input type names** (and nested shapes) depend on the model. Those names are built as:

`strings.ToUpper(<name> + "<Suffix>")`

where `<name>` is **not** always the same helper:

| Query field                                     | `name` passed to `BuildFilterArgument`                                | Rationale                                                          |
| ----------------------------------------------- | --------------------------------------------------------------------- | ------------------------------------------------------------------ |
| **List** (`…List`)                              | `GraphQLTypeNameForFilterArg(stored)` = `ListGraphQLTypeName(stored)` | Same Pascal string as the list element type, e.g. `FoodOrderList`. |
| **Count** (`…ListCount`)                        | `GraphQLComposedTypeName(stored, "List_Count")`                       | Composed segments, e.g. `Food_Order_List_Count`.                   |
| **Nested** (e.g. relation list on another type) | Varies (e.g. `MultipleResourceName(parent + "_" + related)`)          | See `public_schema_builder_build.go` around connection fields.     |

### List query: `name` = list Pascal (`FoodOrderList`)

Examples (explorer / introspection often show **SCREAMING_SNAKE** for input object names):

| Suffix in code         | Example type name                   |
| ---------------------- | ----------------------------------- |
| `_Input_Where_Payload` | `FOODORDERLIST_INPUT_WHERE_PAYLOAD` |
| `_Input_Sort_Payload`  | `FOODORDERLIST_INPUT_SORT_PAYLOAD`  |
| `_Key_Condition`       | `FOODORDERLIST_KEY_CONDITION`       |
| `_GroupBy_Input`       | `FOODORDERLIST_GROUPBY_INPUT`       |
| `_Or_Condition`        | `FOODORDERLIST_OR_CONDITION`        |

**Note:** `strings.ToUpper` is applied to a **single Pascal token** for the list name (`FoodOrderList`), so the uppercase form has **no underscores between** `FOOD`, `ORDER`, and `LIST`—unlike composed names below.

### Count query: `name` = `GraphQLComposedTypeName(stored, "List_Count")`

Example: `Food_Order_List_Count` →

`FOOD_ORDER_LIST_COUNT_INPUT_WHERE_PAYLOAD`, `FOOD_ORDER_LIST_COUNT_KEY_CONDITION`, etc.

Here uppercase preserves **underscores** between segments.

## Connection filter: different `name` source

`BuildConnectionArguments` in `open-core/schemas/objects/search_filter_arg.go` uses:

`strings.ToUpper(stored + "_Connection_Filter_Condition")`

The **`stored` string is passed through as-is** (typically **canonical snake_case**), **not** the list Pascal name.

| Example stored       | Connection filter input type             |
| -------------------- | ---------------------------------------- |
| `food_order`         | `FOOD_ORDER_CONNECTION_FILTER_CONDITION` |
| `foodorder` (legacy) | `FOODORDER_CONNECTION_FILTER_CONDITION`  |

So **list** where-types (`FOODORDERLIST_*`) and **connection** filter types (`FOOD_ORDER_*`) **differ** when the stored id uses underscores: list types collapse the list Pascal token; connection types keep snake-derived word boundaries.

## Legacy vs canonical (explorer screenshots)

| Pattern                                         | Meaning                                                                      |
| ----------------------------------------------- | ---------------------------------------------------------------------------- |
| `Foodorder`, `FOODORDERLIST_*`                  | Legacy **run-on** or single-segment ids; inner words may not be title-cased. |
| `FoodOrder`, `FoodOrderList`, `FOODORDERLIST_*` | Current **Pascal** list/single types with canonical multi-word ids.          |
| `Food_Order_*`, `FOOD_ORDER_*` connection       | **Composed** types and **connection** args from **snake_case** `stored`.     |

After storing **`food_order`** and rebuilding the schema, expect **Pascal** `FoodOrder` / `FoodOrderList` and connection enums **`FOOD_ORDER_*`**, not mixed `FOODORDER` for connection when the stored id contains underscores.

## Optional maintenance

Replace **explorer screenshots** in other documentation after migrating models to canonical ids so UI matches this matrix.

## See also

- `open-core/utility/apito_naming.go` — `PascalFromAnyModelID`, `ListGraphQLTypeName`, `GraphQLComposedTypeName`, `GraphQLTypeNameForFilterArg`
- `open-core/controller/public_schema_builder_build.go` — query/mutation registration
- `open-core/schemas/objects/search_filter_arg.go` — `BuildFilterArgument`, `BuildConnectionArguments`
