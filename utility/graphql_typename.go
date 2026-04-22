package utility

import (
	"strings"
)

// GraphQLTypeName formats a name for GraphQL object/input types.
// It applies Unicode title case to each underscore-separated segment so compound
// names match conventional GraphQL PascalCase (e.g. tag_Update_Payload → Tag_Update_Payload).
// A plain cases.Title on the full string only capitalizes the first rune of the first “word”,
// which yields Tag_update_payload and breaks clients expecting Tag_Update_Payload.
func GraphQLTypeName(s string) string {
	if s == "" {
		return s
	}
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = englishTitleWord(p)
	}
	return strings.Join(parts, "_")
}
