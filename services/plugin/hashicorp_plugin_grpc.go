package plugin

import (
	"context"
	"fmt"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/types/protobuff"
	hcplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

// HashiCorpNormalPluginGRPC is the gRPC implementation for normal plugins
type HashiCorpNormalPluginGRPC struct {
	hcplugin.NetRPCUnsupportedPlugin
	client protobuff.PluginServiceClient
	broker *hcplugin.GRPCBroker
	Impl   interfaces.HashiCorpPluginInterface
}

func (p *HashiCorpNormalPluginGRPC) Init(ctx context.Context, env []*protobuff.EnvVariable) error {
	req := &protobuff.InitRequest{
		EnvVars: env,
	}
	resp, err := p.client.Init(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("plugin init failed: %s", resp.Message)
	}
	return nil
}

func (p *HashiCorpNormalPluginGRPC) Migration(ctx context.Context) error {
	req := &protobuff.MigrationRequest{}
	resp, err := p.client.Migration(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("plugin migration failed: %s", resp.Message)
	}
	return nil
}

func (p *HashiCorpNormalPluginGRPC) SchemaRegister(ctx context.Context) (*protobuff.ThirdPartyGraphQLSchemas, error) {
	req := &protobuff.SchemaRegisterRequest{}
	resp, err := p.client.SchemaRegister(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Schema, nil
}

func (p *HashiCorpNormalPluginGRPC) RESTApiRegister(ctx context.Context) ([]*protobuff.ThirdPartyRESTApi, error) {
	req := &protobuff.RESTApiRegisterRequest{}
	resp, err := p.client.RESTApiRegister(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Apis, nil
}

func (p *HashiCorpNormalPluginGRPC) GetVersion(ctx context.Context) (string, error) {
	req := &protobuff.GetVersionRequest{}
	resp, err := p.client.GetVersion(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Version, nil
}

// Execute calls the plugin's Execute method for function/resolver execution
func (p *HashiCorpNormalPluginGRPC) Execute(ctx context.Context, functionName, functionType string, args map[string]interface{}, contextData map[string]interface{}) (*protobuff.ExecuteResponse, error) {
	// Convert args to protobuf struct
	var argsStruct *structpb.Struct
	var err error
	if args != nil {
		argsStruct, err = structpb.NewStruct(args)
		if err != nil {
			return nil, fmt.Errorf("failed to convert args to struct: %v", err)
		}
	}

	// Convert context data to protobuf struct
	var contextStruct *structpb.Struct
	if contextData != nil {
		contextStruct, err = structpb.NewStruct(contextData)
		if err != nil {
			return nil, fmt.Errorf("failed to convert context to struct: %v", err)
		}
	}

	req := &protobuff.ExecuteRequest{
		FunctionName: functionName,
		FunctionType: functionType,
		Args:         argsStruct,
		Context:      contextStruct,
	}

	resp, err := p.client.Execute(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GRPCServer implementation
func (p *HashiCorpNormalPluginGRPC) GRPCServer(broker *hcplugin.GRPCBroker, s *grpc.Server) error {
	protobuff.RegisterPluginServiceServer(s, &HashiCorpNormalPluginGRPCServer{
		Impl:   p.Impl,
		broker: broker,
	})
	return nil
}

// GRPCClient implementation
func (p *HashiCorpNormalPluginGRPC) GRPCClient(ctx context.Context, broker *hcplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &HashiCorpNormalPluginGRPC{
		client: protobuff.NewPluginServiceClient(c),
		broker: broker,
	}, nil
}

// HashiCorpNormalPluginGRPCServer is the gRPC server implementation for normal plugins
type HashiCorpNormalPluginGRPCServer struct {
	protobuff.UnimplementedPluginServiceServer
	Impl   interfaces.HashiCorpPluginInterface
	broker *hcplugin.GRPCBroker
}

func (s *HashiCorpNormalPluginGRPCServer) Init(ctx context.Context, req *protobuff.InitRequest) (*protobuff.InitResponse, error) {
	// Convert protobuf env vars to extensions env vars
	envVars := ConvertEnvVariablesFromProto(req.EnvVars)
	err := s.Impl.Init(ctx, envVars)
	if err != nil {
		return &protobuff.InitResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}
	return &protobuff.InitResponse{
		Success: true,
		Message: "Plugin initialized successfully",
	}, nil
}

func (s *HashiCorpNormalPluginGRPCServer) Migration(ctx context.Context, req *protobuff.MigrationRequest) (*protobuff.MigrationResponse, error) {
	err := s.Impl.Migration(ctx)
	if err != nil {
		return &protobuff.MigrationResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}
	return &protobuff.MigrationResponse{
		Success: true,
		Message: "Migration completed successfully",
	}, nil
}

func (s *HashiCorpNormalPluginGRPCServer) SchemaRegister(ctx context.Context, req *protobuff.SchemaRegisterRequest) (*protobuff.SchemaRegisterResponse, error) {
	schema, err := s.Impl.SchemaRegister(ctx)
	if err != nil {
		return nil, err
	}

	// Convert schema to protobuf format using helper function
	protoSchema, err := ConvertGraphQLSchemaToProto(schema)
	if err != nil {
		return nil, err
	}

	return &protobuff.SchemaRegisterResponse{
		Schema: protoSchema,
	}, nil
}

func (s *HashiCorpNormalPluginGRPCServer) RESTApiRegister(ctx context.Context, req *protobuff.RESTApiRegisterRequest) (*protobuff.RESTApiRegisterResponse, error) {
	apis, err := s.Impl.RESTApiRegister(ctx)
	if err != nil {
		return nil, err
	}

	// Convert APIs to protobuf format using helper function
	protoApis := ConvertRestApisToProto(apis)

	return &protobuff.RESTApiRegisterResponse{
		Apis: protoApis,
	}, nil
}

func (s *HashiCorpNormalPluginGRPCServer) GetVersion(ctx context.Context, req *protobuff.GetVersionRequest) (*protobuff.GetVersionResponse, error) {
	// Return a default version since this is not part of the plugin interface
	// In practice, this would be configured per plugin
	return &protobuff.GetVersionResponse{
		Version: "1.0.0-universal-plugin",
	}, nil
}
