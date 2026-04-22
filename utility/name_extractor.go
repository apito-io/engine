package utility

import (
	"fmt"
	"github.com/fatih/camelcase"
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/inflection"
	"regexp"
	"strings"
)

// CamelFromAny converts stored model id (canonical snake_case or legacy lowerCamelCase) to lowerCamelCase.
func CamelFromAny(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return ""
	}
	if strings.Contains(modelID, "_") {
		return CamelFromCanonical(modelID)
	}
	return strcase.ToLowerCamel(modelID)
}

var camel = regexp.MustCompile("(^[^A-Z]*|[A-Z]*)([A-Z][^A-Z]+|$)")

// separate update,delete,create from model name
func ExtractResourceName(name string) string {
	if strings.HasSuffix(name, "List") {
		name = strings.TrimSuffix(strings.TrimSpace(name), "List")
	}
	splitted := camelcase.Split(name)
	return strcase.ToLowerCamel(strings.Join(splitted[1:len(splitted)], ""))
}

func ExtractActionName(name string) string {
	splitted := camelcase.Split(name)
	return strcase.ToLowerCamel(splitted[0])
}

func MultipleResourceName(name string) string {
	return fmt.Sprintf("%sList", SingularResourceName(name))
}

// SingularResourceName returns the lowerCamelCase singular resource id for GraphQL fields.
// It accepts canonical snake_case model ids, legacy camelCase ids, or field names ending in List / ListCount.
func SingularResourceName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasSuffix(name, "ListCount") {
		name = strings.TrimSuffix(name, "ListCount")
	} else if strings.HasSuffix(name, "List") {
		name = strings.TrimSuffix(name, "List")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return CamelFromAny(name)
}

// ResolveStoredModelID maps a GraphQL singular base name (e.g. foodCategory from foodCategoryList, or
// food_category) to the project schema's stored model id (e.g. food_category). knownIDs keys are
// Model.Name values. Matches exact id or the same CamelFromAny identity.
func ResolveStoredModelID(knownIDs map[string]bool, graphqlSingular string) string {
	graphqlSingular = strings.TrimSpace(graphqlSingular)
	if graphqlSingular == "" {
		return ""
	}
	if knownIDs[graphqlSingular] {
		return graphqlSingular
	}
	target := CamelFromAny(graphqlSingular)
	for id := range knownIDs {
		if CamelFromAny(id) == target {
			return id
		}
	}
	return ""
}

func ExtractRelationName(name string) string {
	if strings.Contains(name, "_") { // certainly a known_as relation node
		_val := strings.Split(name, "_") // approvalDoctor_practitioner we have to take the "practitioner"
		return strings.TrimSpace(inflection.Singular(strcase.ToLowerCamel(_val[1])))
	}
	return name
}
