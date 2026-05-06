package utility

import (
	"reflect"

	"github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

func GetUserFieldType(fieldInfo *models.FieldInfo) reflect.Kind {
	switch fieldInfo.InputType {
	case _const.StringInput:
		switch fieldInfo.FieldType {
		case _const.ListField:
			if fieldInfo.Validation != nil && (fieldInfo.Validation.IsMultiChoice || len(fieldInfo.Validation.FixedListElements) == 0) { // and multi-choice & dynamic list
				return reflect.Slice
			} else {
				return reflect.String
			}
		case _const.MediaField:
		case _const.MultilineField:
		default:
			return reflect.String
		}
	case _const.IntInput:
		return reflect.Int
	case _const.DoubleInput:
		return reflect.Float64
	case _const.BoolInput:
		return reflect.Bool
	}
	return reflect.Interface
}
