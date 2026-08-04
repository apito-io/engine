package utility

import (
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

func TestFieldIsSortable(t *testing.T) {
	cases := []struct {
		name string
		f    *models.FieldInfo
		want bool
	}{
		{"nil", nil, false},
		{"text", &models.FieldInfo{FieldType: _const.TextField}, true},
		{"number", &models.FieldInfo{FieldType: _const.NumberField, InputType: _const.IntInput}, true},
		{"boolean", &models.FieldInfo{FieldType: _const.BooleanField}, true},
		{"date", &models.FieldInfo{FieldType: _const.DateField}, true},
		{"multiline", &models.FieldInfo{FieldType: _const.MultilineField, Identifier: "bio"}, false},
		{"media", &models.FieldInfo{FieldType: _const.MediaField}, false},
		{"object", &models.FieldInfo{FieldType: _const.ObjectField}, false},
		{"repeated", &models.FieldInfo{FieldType: _const.RepeatedField}, false},
		{"geo", &models.FieldInfo{FieldType: _const.GeoField}, false},
		{
			"list single",
			&models.FieldInfo{
				FieldType: _const.ListField,
				Validation: &models.Validation{
					FixedListElements: []interface{}{"a", "b"},
					IsMultiChoice:     false,
				},
			},
			true,
		},
		{
			"list multi",
			&models.FieldInfo{
				FieldType: _const.ListField,
				Validation: &models.Validation{
					FixedListElements: []interface{}{"a", "b"},
					IsMultiChoice:     true,
				},
			},
			false,
		},
		{"list empty", &models.FieldInfo{FieldType: _const.ListField}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FieldIsSortable(tc.f); got != tc.want {
				t.Fatalf("FieldIsSortable(%s)=%v want %v", tc.name, got, tc.want)
			}
		})
	}
}
