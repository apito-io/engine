package utility

import (
	"testing"

	"github.com/apito-io/engine/models"
)

func TestGetGraphQLObjectSkipsJSONDash(t *testing.T) {
	obj, err := GetGraphQLObject(models.Role{})
	if err != nil {
		t.Fatal(err)
	}
	fields := obj.Fields()
	if _, ok := fields["-"]; ok {
		t.Fatal(`json:"-" must not become GraphQL field "-"`)
	}
	if _, ok := fields["api_permissions"]; !ok {
		t.Fatal("expected api_permissions field on Role")
	}
}
