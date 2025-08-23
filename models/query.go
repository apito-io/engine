package models

import (
	"github.com/vektah/gqlparser/v2/ast"
	"reflect"
)

type QueryFilter struct {
	KeyWrapperFunction     string      `json:"key_wrapper_function"`     // LOWER(x.name)
	Variable               string      `json:"variable"`                 // x
	Key                    string      `json:"key"`                      // name
	Condition              string      `json:"condition"`                // ==
	Value                  interface{} `json:"value"`                    // fahim
	ComplexPredefinedQuery string      `json:"complex_predefined_query"` // for array filter -> COUNT(array[* FILTER CONTAINS(name, CURRENT)])
	IgnoreValue            bool        `json:"ignore_value"`             // for sub query or IntersectIDs sometime we need to ignore the value ex: NOT IN IntersectIDs null
}

type DBPaginationFilter struct {
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

type QueryBuilder struct {
	RawFilterData map[string]interface{}
	DefaultModel  *ModelType

	UserID                   string   `json:"user_id"`
	ProjectID                string   `json:"project_id"`
	RootCollectionFilterType string   `json:"doc_filter_type"`
	DocumentID               string   `json:"document_id"`
	DocumentIDs              []string `json:"document_ids"`

	ParentVariableName string `json:"parent_variable_name"`
	VariablePrefix     string `json:"variable_prefix"`
	VariableName       string `json:"variable_name"`
	CollectionName     string `json:"main_collection_name"`

	RelationCollectionName string               `json:"relation_collection_name"`
	RelationConnection     *ConnectionType      `json:"relation_connection"`
	RelationWhereFilter    []*FilterInformation `json:"relation_where_filter"`

	DefaultFilterCondition string `json:"filter_condition"`

	WhereFilter      []*FilterInformation `json:"where_filter"`   // filter by the user
	DefaultFilter    []*FilterInformation `json:"default_filter"` // default filter that needs to be appended like _key or type of meta.status == 'published'
	SortFilter       []*QueryFilter       `json:"sort_and_limit_param"`
	PaginationFilter *DBPaginationFilter  `json:"limit_filter"`
	FilterByLocal    string               `json:"local"`
	FilterByStatus   string               `json:"status"`

	GroupByFilter map[string]interface{} `json:"group_by_filter"` // used for colelct or group by results

	ConnectionFilter map[string]interface{} `json:"connection_filter"`

	ApitoFields []*FieldInfo `json:"apito_fields"`

	//QueryFilters  []*FilterInformation     `json:"query_filters"`
	SubQueries      []*QueryBuilder          `json:"sub_queries"`
	NestedQueries   []*QueryBuilder          `json:"nested_queries"`
	ReturnFields    map[string]*FieldDetails `json:"return_fields"`
	ReturnOverwrite string                   `json:"return_overwrite"`

	ReturnFieldsSelection *ast.SelectionSet `json:"return_fields_selection"`

	IncludeDefaultSortAndLimit bool `json:"include_default_sort_and_limit"`
	IntersectResult            bool `json:"intersect_result"`
	FetchRevisionDocumentsOnly bool `json:"fetch_revision_documents_only"`
	IsDataloaderQuery          bool `json:"is_dataloader_query"`

	GroupByVariable1 string `json:"group_by_variable1"`
	GroupByVariable2 string `json:"group_by_variable2"`

	IsSystemRequest         bool `json:"is_system_query"`
	IsSystemCollectionQuery bool `json:"is_system_collection_query"`
	IsEntireCollectionQuery bool `json:"is_entire_collection_query"`

	SkipSort          bool `json:"skip_sort"`
	SkipPagination    bool `json:"skip_limit"`
	SkipWhereFilter   bool `json:"skip_where_filter"`
	SkipDefaultFilter bool `json:"skip_default_filter"`

	ReturnOnlyID        bool `json:"return_only_id"`
	ReturnOnlyCount     bool `json:"return_only_count"`
	IsAggregateQuery    bool `json:"is_aggregate_query"`

	ProjectType ProjectType `json:"project_type"`

	FinalQuery string `json:"final_query"`

	TenantID    string `json:"tenant_id"`
	TenantModel string `json:"tenant_model"`
}

type FilterInformation struct {
	Condition string         `json:"condition"`
	Filters   []*QueryFilter `json:"filters"`
}

type FieldDetails struct {
	Identifier string
	Kind       reflect.Kind
	SubFields  []*FieldInfo
	Local      string

	FieldType  string
	Validation *Validation
	Value      interface{}
}
