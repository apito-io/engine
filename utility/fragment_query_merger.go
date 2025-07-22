package utility

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

// Helper function to flatten a SelectionSet by replacing fragment spreads
func flattenSelectionSet(selectionSet ast.SelectionSet, fragments map[string]*ast.FragmentDefinition) ast.SelectionSet {
	var newSelectionSet ast.SelectionSet

	for _, sel := range selectionSet {
		switch s := sel.(type) {
		case *ast.Field:
			// Process nested fields recursively
			if len(s.SelectionSet) > 0 {
				s.SelectionSet = flattenSelectionSet(s.SelectionSet, fragments)
			}
			newSelectionSet = append(newSelectionSet, s)

		case *ast.FragmentSpread:
			// Replace fragment spread with the actual fields from the fragment
			fragment := fragments[s.Name]
			if fragment != nil {
				// Recursively flatten the fragment's SelectionSet
				flattenedFragment := flattenSelectionSet(fragment.SelectionSet, fragments)
				newSelectionSet = append(newSelectionSet, flattenedFragment...)
			}

		case *ast.InlineFragment:
			// Inline fragment, handle like a normal SelectionSet
			if len(s.SelectionSet) > 0 {
				flattenedInline := flattenSelectionSet(s.SelectionSet, fragments)
				newSelectionSet = append(newSelectionSet, flattenedInline...)
			}
		}
	}

	return newSelectionSet
}

// Helper function to convert a SelectionSet to a string representation
func selectionSetToString(selectionSet ast.SelectionSet, indent int) string {
	var builder strings.Builder
	indentation := strings.Repeat("  ", indent)

	for _, selection := range selectionSet {
		switch s := selection.(type) {
		case *ast.Field:
			builder.WriteString(indentation + s.Name)

			// Include field arguments if present
			if len(s.Arguments) > 0 {
				builder.WriteString("(")
				for i, arg := range s.Arguments {
					if i > 0 {
						builder.WriteString(", ")
					}
					builder.WriteString(fmt.Sprintf("%s: %s", arg.Name, arg.Value.String()))
				}
				builder.WriteString(")")
			}

			if len(s.SelectionSet) > 0 {
				builder.WriteString(" {\n")
				builder.WriteString(selectionSetToString(s.SelectionSet, indent+1))
				builder.WriteString(indentation + "}\n")
			} else {
				builder.WriteString("\n")
			}
		}
	}

	return builder.String()
}

// New function to generate the combined output as a single string
func GenerateCombinedQuery(doc *ast.QueryDocument, fragments map[string]*ast.FragmentDefinition) string {
	var builder strings.Builder

	for _, operation := range doc.Operations {
		// Detect whether it's a query or mutation
		operationType := operation.Operation
		if operationType == "" {
			operationType = "query" // Default to query if not specified
		}

		// Start building the query/mutation string
		builder.WriteString(fmt.Sprintf("%s %s(", operationType, operation.Name))
		if len(operation.VariableDefinitions) > 0 {
			// Include any variable definitions dynamically
			for i, variable := range operation.VariableDefinitions {
				if i > 0 {
					builder.WriteString(", ")
				}
				builder.WriteString(fmt.Sprintf("$%s: %s", variable.Variable, variable.Type.String()))
			}
		}
		builder.WriteString(") {\n")

		// Flatten the SelectionSet
		operation.SelectionSet = flattenSelectionSet(operation.SelectionSet, fragments)

		// Append the flattened SelectionSet to the output
		builder.WriteString(selectionSetToString(operation.SelectionSet, 1))
		builder.WriteString("}\n")
	}

	return builder.String()
}
