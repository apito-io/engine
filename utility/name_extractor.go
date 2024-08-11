package utility

import (
	"github.com/fatih/camelcase"
	"github.com/iancoleman/strcase"
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
	splitted := camelcase.Split(name)
	return strcase.ToLowerCamel(strings.Join(splitted[1:len(splitted)], ""))
}

func ExtractActionName(name string) string {
	splitted := camelcase.Split(name)
	return strcase.ToLowerCamel(splitted[0])
}
