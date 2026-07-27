package executor

import (
	"context"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

func TestHandlePayloadFormatting_repeatedField_emptyClearsOnFullReplace(t *testing.T) {
	s := &GraphQLExecutor{}
	field := &models.FieldInfo{
		Identifier: "sections",
		FieldType:  _const.RepeatedField,
		SubFieldInfo: []*models.FieldInfo{
			{Identifier: "code", FieldType: _const.TextField},
			{Identifier: "name", FieldType: _const.TextField},
		},
	}

	existing := map[string]interface{}{
		"sections": []interface{}{
			map[string]interface{}{"_id": "s1", "code": "A", "name": "A"},
		},
	}

	t.Run("empty clears when deltaUpdate false", func(t *testing.T) {
		payload := map[string]interface{}{
			"sections": []interface{}{},
		}
		out, err := s.HandlePayloadFormatting(
			context.Background(), nil, "en",
			[]*models.FieldInfo{field}, payload, existing, false,
		)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := out["sections"].([]interface{})
		if !ok || len(got) != 0 {
			t.Fatalf("expected empty sections, got %#v", out["sections"])
		}
	})

	t.Run("empty is no-op when deltaUpdate true", func(t *testing.T) {
		db := map[string]interface{}{
			"sections": []interface{}{
				map[string]interface{}{"_id": "s1", "code": "A", "name": "A"},
			},
		}
		payload := map[string]interface{}{
			"sections": []interface{}{},
		}
		out, err := s.HandlePayloadFormatting(
			context.Background(), nil, "en",
			[]*models.FieldInfo{field}, payload, db, true,
		)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := out["sections"].([]interface{})
		if !ok || len(got) != 1 {
			t.Fatalf("expected existing section kept, got %#v", out["sections"])
		}
	})
}
