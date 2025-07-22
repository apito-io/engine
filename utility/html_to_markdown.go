package utility

import (
	"errors"
	"reflect"

	"github.com/apito-io/engine/models"
)

func HandlePayload(model *models.ModelType, data map[string]interface{}) {
	for k, v := range data {
		if v != nil {
			for _, field := range model.Fields {
				/*if k == field.Identifier && field.FieldType == "multiline" {
					val, err := MultiLineTextFormat(v)
					if err != nil {
						fmt.Println(err.Error())
					}
					data[k] = val
					break
				} else */if k == field.Identifier && field.FieldType == "repeated" {
					for _, v := range v.([]interface{}) {
						vv := v.(map[string]interface{})
						HandleSubPayload(field, vv)
					}
				} else if k == field.Identifier && field.FieldType == "object" {
					vv := v.(map[string]interface{})
					HandleSubPayload(field, vv)
				} else if k == field.Identifier && field.InputType == "int" {
					data[k], _ = IntFormat(v)
					break
				}
			}
		}
	}
}

func HandleSubPayload(field *models.FieldInfo, data map[string]interface{}) {
	for k, v := range data {
		if v != nil {
			for _, field := range field.SubFieldInfo {
				/*if k == field.Identifier && field.FieldType == "multiline" {
					val, err := MultiLineTextFormat(v)
					if err != nil {
						fmt.Println(err.Error())
					}
					data[k] = val
					break
				} else*/if k == field.Identifier && field.InputType == "int" {
					data[k], _ = IntFormat(v)
					break
				}
			}
		}
	}
}

func MultiLineTextFormat(v interface{}) (map[string]interface{}, error) {
	var val map[string]interface{}
	var err error
	switch reflect.TypeOf(v).Kind() {
	case reflect.Map:
		val = v.(map[string]interface{})
		if html, ok := val["html"].(string); ok && html != "" {
			//converter := md.NewConverter("", true, nil)
			//markdown, err := converter.ConvertString(html)
			/*markdown := html2md.Convert(html)
			if err != nil {
				return nil, err
			}
			val["markdown"] = markdown
			text := html2text.HTML2Text(html)
			val["text"] = text*/
		}
	case reflect.Slice:
		datas := v.([]interface{})
		for _, value := range datas {
			if mapped, ok := value.(map[string]interface{}); ok {
				for _, v := range mapped {
					val, err = MultiLineTextFormat(v)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return val, err
}

func IntFormat(v interface{}) (int, error) {
	_int := int(v.(float64))
	if _int > 2147483647 {
		return 0, errors.New("int Value Overflow")
	}
	return _int, nil
}
