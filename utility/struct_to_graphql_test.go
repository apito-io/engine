package utility

import (
	"testing"

	"github.com/apito-io/engine/models"
)

func TestSum(t *testing.T) {
	CloudFunctionType, err := GetGraphQLObject(models.ApitoFunction{})
	if err != nil {
		t.Error(err)
	}
	t.Log(CloudFunctionType)
}
