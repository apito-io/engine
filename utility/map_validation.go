package utility

import (
	"fmt"
	"reflect"
	"strings"
)

func valueFormater(_val interface{}) interface{} {
	switch reflect.ValueOf(_val).Kind() {
	case reflect.String:
		return fmt.Sprintf(`"%v"`, _val)
	case reflect.Map:
		v := _val.(map[string]interface{})
		var values []string
		for k, v := range v {
			values = append(values, fmt.Sprintf("%s : %v", k, valueFormater(v)))
		}
		return fmt.Sprintf(`{ %s }`, strings.Join(values, ", "))
	case reflect.Slice:
		for _, v := range _val.([]interface{}) {
			vv := v.(map[string]interface{})
			var values []string
			for k, v := range vv {
				values = append(values, fmt.Sprintf("%s : %v", k, valueFormater(v)))
			}
			return fmt.Sprintf(`{ %s }`, strings.Join(values, ", "))
		}
	default:
		return _val
	}
	return nil
}

func validPayloadBuilder(reqPayload map[string]interface{}, validFields map[string]interface{}, validConnections map[string]string) (string, string) {
	var connections []string
	var payloads []string
	for k, _val := range reqPayload {
		if field, ok := validFields[k]; ok || k == "_id" {
			// validate if nested
			if field != nil {
				reqValue := reqPayload[k]
				switch reflect.ValueOf(reqValue).Kind() {
				case reflect.Slice:
					nestedVals := reqValue.([]interface{})
					for _, nestedValue := range nestedVals {
						nv := nestedValue.(map[string]interface{})
						fs := field.(map[string]interface{})
						validPayloadBuilder(nv, fs, nil) // for nested there wont be any connections
					}
					break
				case reflect.Map:
					nestedValue := reqValue.(map[string]interface{})
					for k, v := range nestedValue {
						if contains(field.([]string), k) || k == "_id" {
							fmt.Println(k, v)
						} else {
							delete(nestedValue, k)
						}
					}
				}
			}
			payloads = append(payloads, fmt.Sprintf(`%s : %v`, k, valueFormater(_val)))
		} else if k == "_connections" {
			if val, ok := _val.(map[string]interface{}); ok {
				for k, givenRelations := range val {
					sps := strings.Split(k, "_")
					if relation, ok := validConnections[sps[0]]; ok {
						switch relation {
						case "has_many":
							if reflect.ValueOf(givenRelations).Kind() == reflect.Slice {
								inputConnections := givenRelations.([]interface{})
								if len(inputConnections) > 0 {
									var vals []string
									for _, v := range inputConnections {
										vals = append(vals, v.(string))
									}
									connections = append(connections, fmt.Sprintf(`%s: ["%s"]`, k, strings.Join(vals, `","`)))
								}
							}
							break
						case "has_one":
							if reflect.ValueOf(givenRelations).Kind() == reflect.String {
								id := givenRelations.(string)
								if id != "" {
									connections = append(connections, fmt.Sprintf(`%s : "%s"`, k, id))
								}
							}
							break
						}
					}
				}
			}
		} else {
			delete(reqPayload, k)
		}
	}
	return strings.Join(payloads, ", "), strings.Join(connections, ", ")
}
