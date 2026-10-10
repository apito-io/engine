package executor

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
)

func TestHandlePayloadFormatting_listField_subjectCodes(t *testing.T) {
	s := &GraphQLExecutor{}
	field := &models.FieldInfo{
		Identifier: "subject_codes",
		FieldType:  "list",
		InputType:  "string",
	}

	t.Run("slice interface", func(t *testing.T) {
		payload := map[string]interface{}{
			"subject_codes": []interface{}{"101", "102"},
		}
		out, err := s.HandlePayloadFormatting(context.Background(), nil, "en", []*models.FieldInfo{field}, payload, make(map[string]interface{}), false)
		if err != nil {
			t.Fatal(err)
		}
		if out["subject_codes"] == nil {
			t.Fatalf("expected subject_codes stored, got nil; out=%v", out)
		}
	})

	t.Run("fixed list typed string slice", func(t *testing.T) {
		f := &models.FieldInfo{
			Identifier: "skin_condition",
			FieldType:  "list",
			InputType:  "string",
			Validation: &models.Validation{
				FixedListElements: []interface{}{"dry", "normal", "rash", "ulcer"},
			},
		}
		payload := map[string]interface{}{
			"skin_condition": []string{"dry"},
		}
		out, err := s.HandlePayloadFormatting(context.Background(), nil, "en", []*models.FieldInfo{f}, payload, make(map[string]interface{}), false)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := out["skin_condition"].([]string)
		if len(got) != 1 || got[0] != "dry" {
			t.Fatalf("skin_condition=%v", out["skin_condition"])
		}
	})

	t.Run("slice string", func(t *testing.T) {
		payload := map[string]interface{}{
			"subject_codes": []string{"101", "102"},
		}
		out, err := s.HandlePayloadFormatting(context.Background(), nil, "en", []*models.FieldInfo{field}, payload, make(map[string]interface{}), false)
		if err != nil {
			t.Fatal(err)
		}
		if out["subject_codes"] == nil {
			t.Fatalf("expected subject_codes stored, got nil; out=%v", out)
		}
	})

	t.Run("validation empty fixed list elements", func(t *testing.T) {
		f := &models.FieldInfo{
			Identifier: "subject_codes",
			FieldType:  "list",
			InputType:  "string",
			Validation: &models.Validation{FixedListElements: []interface{}{}},
		}
		payload := map[string]interface{}{
			"subject_codes": []interface{}{"101", "102"},
		}
		out, err := s.HandlePayloadFormatting(context.Background(), nil, "en", []*models.FieldInfo{f}, payload, make(map[string]interface{}), false)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("out subject_codes=%v", out["subject_codes"])
	})
}
