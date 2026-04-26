package models

// PublicSchemaModelFilter is one model (plus optional request filter metadata)
// participating in a public GraphQL schema build.
type PublicSchemaModelFilter struct {
	Model             *ModelType
	Filter            *FilteredModel
	HasMetaQuery      bool
	IsDataloaderModel bool
	KnownAs           string
}
