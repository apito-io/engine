package utility

import (
	"fmt"
	"github.com/fatih/camelcase"
	"github.com/iancoleman/strcase"
	"github.com/jinzhu/inflection"
	"regexp"
	"strings"
)

var camel = regexp.MustCompile("(^[^A-Z]*|[A-Z]*)([A-Z][^A-Z]+|$)")

func splitCameCase(s string) []string {
	var results []string
	for _, sub := range camel.FindAllStringSubmatch(s, -1) {
		results = sub
	}
	return results
}

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
	//return inflection.Plural(name)
	return fmt.Sprintf("%sList", SingularResourceName(name))
}

func SingularResourceName(name string) string {
	if strings.HasSuffix(name, "List") {
		return strings.TrimSuffix(strings.TrimSpace(inflection.Singular(strcase.ToLowerCamel(name))), "List")
	}
	if strings.HasSuffix(name, "ListCount") {
		return strings.TrimSuffix(strings.TrimSpace(inflection.Singular(strcase.ToLowerCamel(name))), "ListCount")
	}
	return strings.TrimSpace(inflection.Singular(strcase.ToLowerCamel(name)))
}

func ExtractRelationName(name string) string {
	if strings.Contains(name, "_") { // certainly a known_as relation node
		_val := strings.Split(name, "_") // approvalDoctor_practitioner we have to take the "practitioner"
		return strings.TrimSpace(inflection.Singular(strcase.ToLowerCamel(_val[1])))
	}
	return name
}
