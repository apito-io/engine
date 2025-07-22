package interfaces

import (
	"context"

	"github.com/apito-io/types/protobuff"
)

// HashiCorpPluginInterface interface functions for HashiCorp go-plugin system
type HashiCorpPluginInterface interface {
	// Init This functions runs when any extension runs
	Init(ctx context.Context, env []*protobuff.EnvVariable) error

	// Migration If your extension has any migration script you can put it here
	Migration(ctx context.Context) error

	// SchemaRegister Define the GraphQL Schema That Will be Added If this extension registers
	SchemaRegister(ctx context.Context) (*protobuff.ThirdPartyGraphQLSchemas, error)

	// RESTApiRegister Define the REST api that will be added to the existing list
	RESTApiRegister(ctx context.Context) ([]*protobuff.ThirdPartyRESTApi, error)

	// Execute calls when the function is called
	Execute(ctx context.Context, request interface{}) (interface{}, error)
}
