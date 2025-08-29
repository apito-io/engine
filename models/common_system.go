package models

import (
	"context"
	"encoding/json"

	"github.com/apito-io/types"
	dlv6 "github.com/graph-gophers/dataloader"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/tailor-inc/graphql"
	"github.com/vektah/gqlparser/v2/ast"
)

type SearchResponse[T any] struct {
	Results        []*T
	GroupedResults map[string][]*T
}

// DataLoaders Dataloaders
type DataLoaders struct {
	MultiLoader *dataloader.Loader[string, interface{}]
	//SingleLoader *dataloader.Loader[string, interface{}]
}

type XResponse struct {
	Data       interface{}            `json:"data,omitempty"`
	Errors     json.RawMessage        `json:"errors,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

type RawSchema struct {
	Queries   graphql.Fields
	Mutations graphql.Fields
}

type GraphQLIncomingRequest struct {
	Query         string                 `json:"query" url:"query" schema:"query"`
	Variables     map[string]interface{} `json:"variables" url:"variables" schema:"variables"`
	OperationName string                 `json:"operation_name" url:"operation_name" schema:"operation_name"`
	QueryType     string                 `json:"query_type" url:"query_type" schema:"query_type"` // query, mutation, subscription
}

type ApplicationCache struct {
	Ctx         context.Context         `json:"ctx,omitempty"`
	Project     *Project                `json:"project,omitempty"`
	Param       *CommonSystemParams     `json:"param,omitempty"`
	RawSchemas  *RawSchema              `json:"raw_schema,omitempty"`
	Dataloaders map[string]*dlv6.Loader `json:"dataloaders,omitempty"`

	IncomingRequest []*IncomingRequest      `json:"incoming_request"`
	GraphqlRequest  *GraphQLIncomingRequest `json:"graphql_request,omitempty"`
}

func FilterUniqueStrings(models []*FilteredModel) []*FilteredModel {
	seen := make(map[string]bool)
	var result []*FilteredModel
	for _, s := range models {
		key := s.Name
		if s.KnownAs != "" {
			key = s.KnownAs
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, s)
		}
	}
	return result
}

type FilteredModel struct {
	Name              string
	WhereFilter       []string
	IsConnectionQuery bool
	HasMetaQuery      bool
	KnownAs           string // used in known_as relation node it is equal tographql alias
	IsDataloaderModel bool   // used in dataloader model
}

type IncomingRequest struct {
	OperationType     string
	FilteredModels    []*FilteredModel
	FilteredFunctions []*ApitoFunction
	IsPluginRequest   bool
}

type CommonSystemParams struct {
	Role          *Role  `json:"role,omitempty"`
	Plan          string `json:"plan,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	RelationModel string `json:"relation_model,omitempty"`
	Email         string `json:"email,omitempty"`
	ProjectID     string `json:"project_id,omitempty"`

	// For SaaS project
	TenantID    string `json:"tenant_id,omitempty"` // used in SaaS project user token
	TenantModel string `json:"tenant_model"`        // used in SaaS project user token

	ResolveParams *graphql.ResolveParams `json:"resolve_params,omitempty"`

	SystemCollectionName string `json:"system_collection_name,omitempty"`

	DocumentID  string   `json:"document_id,omitempty"`
	DocumentIDs []string `json:"document_ids,omitempty"`

	Document    *types.DefaultDocumentStructure `json:"document,omitempty"`
	Model       *ModelType                      `json:"model_type,omitempty"`
	ConDisParam []*ConnectDisconnectParam       `json:"con_dis_param,omitempty"`
	FieldInfo   *FieldInfo                      `json:"field_info,omitempty"`

	KnownAs        string `json:"known_as,omitempty"`
	Revision       bool   `json:"revision,omitempty"`
	SinglePageData bool   `json:"single_page_data,omitempty"`

	//Limit *protobuff.UsagesTracking `json:"limit,,omitempty"`

	DocPublishStatus string `json:"doc_publish_status,omitempty"`

	IsSystemRequest                 bool `json:"is_system_request,omitempty"`
	IsEntireCollectionSearchRequest bool `json:"is_entire_collection_search_request,omitempty"`
	IsDataloaderRequest             bool `json:"is_dataloader_request,omitempty"`
	IsIntersectionResult            bool `json:"is_intersection_result,omitempty"`

	// these three used in intersection of two collections
	SkipSort          bool `json:"skip_sort,omitempty"`
	SkipPagination    bool `json:"skip_pagination,omitempty"`
	SkipWhereFilter   bool `json:"skip_filter,omitempty"`         // used in intersection of two collections, where filter is not needed
	SkipDefaultFilter bool `json:"skip_default_filter,omitempty"` // if you want to skip the default filter

	ReturnOnlyID     bool   `json:"return_only_id,omitempty"`
	OnlyReturnCount  bool   `json:"only_return_count,omitempty"`
	IsAggregateQuery bool   `json:"is_aggregate_query,omitempty"`
	ReturnOverwrite  string `json:"return_overwrite,omitempty"`

	QuerySelectionSets *ast.SelectionSet `json:"query_selection_sets,omitempty"`

	UnmarshalStructure interface{} `json:"unmarshal_structure"`

	ProjectType ProjectType `json:"project_type"`
}

type DocumentRevisionHistory struct {
	ID         string `json:"id"`
	RevisionAt string `json:"revision_at"`
	Status     string `json:"status"`
}

type InjectableHasOneConnection struct {
	ModelName string
	IDs       []string
	Data      map[string]string
}

type ConnectDisconnectParam struct {
	DocCollectionName string
	DocRelationName   string

	ActionIDs       []string
	CurrentActionID string // used in single delete relation
	ActionIDType    string // direct, indirect

	ConnectionType      string
	ForwardConnectionID string

	ForwardConnectionType      *ConnectionType
	ForwardConnectionModelType *ModelType

	BackwardConnectionType      *ConnectionType
	BackwardConnectionModelType *ModelType

	KnownAs string

	InjectableHasOneConnects []*InjectableHasOneConnection
}

type EdgeRelation struct {
	XFrom string `json:"_from,omitempty" bson:"_from,omitempty"`
	XTo   string `json:"_to,omitempty" bson:"_to,omitempty"`
	Key   string `json:"_key,omitempty" bson:"_id,omitempty"`

	Relation string `json:"relation,omitempty" bson:"relation,omitempty"`
	From     string `json:"from,omitempty" bson:"from,omitempty"`
	FromID   string `json:"from_id,omitempty" bson:"from_id,omitempty"`
	To       string `json:"to,omitempty" bson:"to,omitempty"`
	ToID     string `json:"to_id,omitempty" bson:"to_id,omitempty"`

	Role        string   `json:"role,omitempty" bson:"role,omitempty"`
	KnownAs     string   `json:"known_as,omitempty" bson:"known_as,omitempty"`
	Permissions []string `json:"permissions,omitempty" bson:"permissions,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty" bson:"created_at,omitempty"`

	TenantID    string `json:"tenant_id,omitempty" bson:"tenant_id,omitempty"`
	TenantModel string `json:"tenant_model,omitempty" bson:"tenant_model,omitempty"`
}

type ModelDocsResponse struct {
	Docs  []*types.DefaultDocumentStructure `json:"docs"`
	Count int                               `json:"count"`
}
