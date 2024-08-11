package utility

import (
	"testing"

	"github.com/apito-io/buffers/protobuff"
)

func TestSum(t *testing.T) {
	CloudFunctionType, err := GetGraphQLObject(protobuff.CloudFunction{})
	if err != nil {
		t.Error(err)
	}
	t.Log(CloudFunctionType)
}
