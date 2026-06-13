package mysql

import (
	"strings"
	"testing"
	"time"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

func TestSelectBuilder_NoUserFields(t *testing.T) {
	parts := SelectBuilder("y", "", &models.ModelType{Name: "author", Fields: nil}, false)
	q := strings.Join(parts, ", ")
	if strings.Contains(q, ", ,") {
		t.Fatalf("double comma in select (invalid SQL): %q", q)
	}
	if !strings.Contains(q, "x.id AS id") {
		t.Fatalf("expected id column: %q", q)
	}
	if !strings.Contains(q, "sys_created_at") {
		t.Fatalf("expected meta columns: %q", q)
	}
}

func TestSelectBuilder_WithFields(t *testing.T) {
	parts := SelectBuilder("y", "", &models.ModelType{
		Name: "author",
		Fields: []*models.FieldInfo{
			{Identifier: "name"},
		},
	}, false)
	q := strings.Join(parts, ", ")
	if strings.Contains(q, ", ,") {
		t.Fatalf("double comma: %q", q)
	}
	if !strings.Contains(q, "x.name AS name") {
		t.Fatalf("expected field projection: %q", q)
	}
}

func TestFormatSQLMetaTimestamp_SQLiteDateString(t *testing.T) {
	s, err := formatSQLMetaTimestamp("2026-04-01")
	if err != nil {
		t.Fatal(err)
	}
	if want := "2026-04-01T00:00:00Z"; s != want {
		t.Fatalf("got %q want %q", s, want)
	}
}

func TestFormatSQLMetaTimestamp_timeTime(t *testing.T) {
	tm := time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC)
	s, err := formatSQLMetaTimestamp(tm)
	if err != nil {
		t.Fatal(err)
	}
	if want := "2026-04-01T12:30:00Z"; s != want {
		t.Fatalf("got %q want %q", s, want)
	}
}

func TestFormatSQLMetaTimestamp_goStringDuplicateOffset(t *testing.T) {
	s, err := formatSQLMetaTimestamp("2026-05-27 11:28:01 +0600 +0600")
	if err != nil {
		t.Fatal(err)
	}
	if want := "2026-05-27T05:28:01Z"; s != want {
		t.Fatalf("got %q want %q", s, want)
	}
}

func TestCommonDocTransformation_dateFieldRFC3339(t *testing.T) {
	classification := &FieldClassification{DateFields: []string{"date"}}
	doc, err := CommonDocTransformation(&models.ModelType{Name: "food_order"}, "en", map[string]interface{}{
		"id":   "abc",
		"date": time.Date(2026, 5, 27, 11, 28, 1, 0, time.FixedZone("", 6*3600)),
	}, classification)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := doc.Data["date"].(string)
	if !ok {
		t.Fatalf("expected string date, got %T", doc.Data["date"])
	}
	if want := "2026-05-27T05:28:01Z"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFilterBuilder_between_stringField(t *testing.T) {
	model := &models.ModelType{
		Name: "food_order",
		Fields: []*models.FieldInfo{
			{Identifier: "date", InputType: _const.StringInput, FieldType: _const.DateField},
		},
	}
	var sqlArgs []interface{}
	preds, err := FilterBuilder("x", map[string]interface{}{
		"date": map[string]interface{}{
			"between": []interface{}{"2026-05-12", "2026-05-13"},
		},
	}, model, &sqlArgs)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 {
		t.Fatalf("expected 1 predicate, got %v", preds)
	}
	if !strings.Contains(preds[0], ">=") || !strings.Contains(preds[0], "<=") {
		t.Fatalf("expected range predicate, got %q", preds[0])
	}
	if len(sqlArgs) != 2 {
		t.Fatalf("expected 2 args, got %v", sqlArgs)
	}
}

func TestConditionBuilder_multipleScalarFields(t *testing.T) {
	model := &models.ModelType{
		Name: "t",
		Fields: []*models.FieldInfo{
			{Identifier: "a", InputType: _const.StringInput, FieldType: _const.TextField},
			{Identifier: "b", InputType: _const.StringInput, FieldType: _const.TextField},
		},
	}
	var sqlArgs []interface{}
	filters, err := ConditionBuilder("x", map[string]interface{}{
		"where": map[string]interface{}{
			"a": map[string]interface{}{"eq": "1"},
			"b": map[string]interface{}{"eq": "2"},
		},
	}, model, &sqlArgs)
	if err != nil {
		t.Fatal(err)
	}
	if g := filters["AND"]; len(g) != 2 {
		t.Fatalf("expected 2 AND predicates, got %v", g)
	}
	if len(sqlArgs) != 2 {
		t.Fatalf("expected 2 bind args, got %v", sqlArgs)
	}
}

func TestRelationEdgeMatchesParentModel_normalizedIds(t *testing.T) {
	parent := "food_order"
	if !relationEdgeMatchesParentModel(&models.ConnectionType{Model: "food_order"}, parent) {
		t.Fatal("exact match should succeed")
	}
	if !relationEdgeMatchesParentModel(&models.ConnectionType{Model: "Food_Order"}, parent) {
		t.Fatal("PhysicalSQLTableName normalization should match legacy casing")
	}
	if relationEdgeMatchesParentModel(&models.ConnectionType{Model: "customer"}, parent) {
		t.Fatal("different model should not match")
	}
	if relationEdgeMatchesParentModel(nil, parent) {
		t.Fatal("nil edge should not match")
	}
}
