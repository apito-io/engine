package utility

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/fatih/camelcase"
	"github.com/jinzhu/inflection"
)

// Naming schema version stored on ProjectSchema.NamingSchemaVersion.
const (
	NamingSchemaVersionLegacy = 0
	NamingSchemaVersionV2     = 1
)

var (
	// ErrRunOnModelName is returned when the input has no word-boundary signal
	// and looks like concatenated words (e.g. foodorder). Users must use
	// food_order, food-order, foodOrder, or "food order".
	ErrRunOnModelName = errors.New("model name needs a word boundary between words: use food_order, food-order, foodOrder, or \"food order\"")

	// ErrInvalidModelName is returned when the normalized id does not match canonical rules.
	ErrInvalidModelName = errors.New("invalid model name: use lowercase snake_case with letters, numbers, and underscores only")

	canonicalIDRegexp = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)

	// singularKeepAsIs are last-segment words that must not be passed through inflection.Singular.
	singularKeepAsIs = map[string]struct{}{
		"news": {}, "data": {}, "media": {}, "analytics": {}, "series": {}, "species": {},
		"users": {}, // project auth user table / hidden schema model id (must not become reserved "user")
	}
)

// IsCanonicalModelID reports whether s is already stored canonical form (snake_case ASCII).
func IsCanonicalModelID(s string) bool {
	return s != "" && canonicalIDRegexp.MatchString(s)
}

// CanonicalizeModelName normalizes admin input into the canonical snake_case singular model id.
func CanonicalizeModelName(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("model name is required")
	}

	// Already-canonical ids (sync, draft replay, stored schema) skip run-on rejection.
	// Long single words like "indication" / "practitioner" are valid snake_case model ids;
	// rejectRunOnLowercaseConcat is only for free-text admin input like "foodorder".
	if IsCanonicalModelID(raw) {
		parts := strings.Split(raw, "_")
		parts[len(parts)-1] = singularizeSegment(parts[len(parts)-1])
		out := strings.Join(parts, "_")
		if !canonicalIDRegexp.MatchString(out) {
			return "", ErrInvalidModelName
		}
		if err := checkReservedModelName(out); err != nil {
			return "", err
		}
		return out, nil
	}

	if err := rejectRunOnLowercaseConcat(raw); err != nil {
		return "", err
	}

	segments, err := splitIntoWordSegments(raw)
	if err != nil {
		return "", err
	}
	if len(segments) == 0 {
		return "", ErrInvalidModelName
	}

	last := len(segments) - 1
	segments[last] = singularizeSegment(segments[last])

	out := strings.Join(segments, "_")
	if !canonicalIDRegexp.MatchString(out) {
		return "", ErrInvalidModelName
	}

	if err := checkReservedModelName(out); err != nil {
		return "", err
	}

	return out, nil
}

// rejectRunOnLowercaseConcat rejects single-token all-lowercase strings that are long
// enough to likely be two words jammed (e.g. foodorder) while allowing short single
// words like "category" (len 8). Threshold: no boundary and len >= 9.
// Not applied when the input is already a canonical model id (see CanonicalizeModelName).
func rejectRunOnLowercaseConcat(raw string) error {
	if strings.ContainsAny(raw, " \t\n\r_-") {
		return nil
	}
	if regexp.MustCompile(`[a-z][A-Z]`).MatchString(raw) {
		return nil
	}
	if !regexp.MustCompile(`^[a-z]+$`).MatchString(raw) {
		return nil
	}
	if len(raw) >= 9 {
		return ErrRunOnModelName
	}
	return nil
}

func splitIntoWordSegments(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "-", "_")
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '_' || unicode.IsSpace(r)
	})
	var segments []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		for _, piece := range camelcase.Split(p) {
			piece = strings.TrimSpace(piece)
			if piece == "" {
				continue
			}
			var b strings.Builder
			for _, r := range piece {
				if unicode.IsLetter(r) || unicode.IsDigit(r) {
					b.WriteRune(unicode.ToLower(r))
				}
			}
			s := b.String()
			if s != "" {
				segments = append(segments, s)
			}
		}
	}
	return segments, nil
}

func singularizeSegment(seg string) string {
	if _, ok := singularKeepAsIs[seg]; ok {
		return seg
	}
	return strings.TrimSpace(inflection.Singular(seg))
}

func checkReservedModelName(canonical string) error {
	switch canonical {
	case "list":
		return errors.New("naming a Model `List` is not allowed. Apito uses List to represent plural of a resource automatically. Try another name instead")
	case "user":
		return errors.New("naming a Model `User` is protected. If you want to store authenticated users. Try adding Authentication module from Settings > Add-Ons")
	case "system":
		return errors.New("naming a Model `System` is not allowed. Try Another alternate name instead")
	case "function":
		return errors.New("naming a Model `Function` is not allowed. Try Another alternate name instead")
	default:
		return nil
	}
}

// LegacyStoredNameToCanonical converts a legacy stored model name (camelCase or run-on)
// to canonical snake_case for migration. Does not apply run-on rejection.
func LegacyStoredNameToCanonical(stored string) (string, error) {
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return "", errors.New("empty model name")
	}
	if IsCanonicalModelID(stored) {
		parts := strings.Split(stored, "_")
		if len(parts) == 0 {
			return "", ErrInvalidModelName
		}
		parts[len(parts)-1] = singularizeSegment(parts[len(parts)-1])
		out := strings.Join(parts, "_")
		if !canonicalIDRegexp.MatchString(out) {
			return "", ErrInvalidModelName
		}
		return out, nil
	}
	segments, err := splitIntoWordSegments(stored)
	if err != nil || len(segments) == 0 {
		return "", fmt.Errorf("cannot migrate model name %q: %w", stored, ErrInvalidModelName)
	}
	segments[len(segments)-1] = singularizeSegment(segments[len(segments)-1])
	out := strings.Join(segments, "_")
	if !canonicalIDRegexp.MatchString(out) {
		return "", ErrInvalidModelName
	}
	return out, nil
}

// CamelFromCanonical converts canonical snake_case to lowerCamelCase (e.g. food_order -> foodOrder).
func CamelFromCanonical(canonical string) string {
	if canonical == "" {
		return ""
	}
	parts := strings.Split(canonical, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i == 0 {
			parts[i] = strings.ToLower(p)
		} else {
			parts[i] = englishTitleWord(p)
		}
	}
	return strings.Join(parts, "")
}

// PascalFromCanonical converts canonical snake_case to PascalCase without underscores (e.g. food_order -> FoodOrder).
func PascalFromCanonical(canonical string) string {
	if canonical == "" {
		return ""
	}
	parts := strings.Split(canonical, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(englishTitleWord(p))
	}
	return b.String()
}

// modelIDSegments returns lowercased word segments for a stored model id (canonical snake or legacy camel).
func modelIDSegments(modelID string) []string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return nil
	}
	if strings.Contains(modelID, "_") {
		var out []string
		for _, p := range strings.Split(modelID, "_") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, strings.ToLower(p))
			}
		}
		return out
	}
	var out []string
	for _, s := range camelcase.Split(modelID) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, strings.ToLower(s))
	}
	return out
}

func appendSuffixSegments(parts []string, suffix string) []string {
	suffix = strings.TrimPrefix(strings.TrimSpace(suffix), "_")
	for _, chunk := range strings.Split(suffix, "_") {
		if chunk == "" {
			continue
		}
		for _, s := range camelcase.Split(chunk) {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			parts = append(parts, strings.ToLower(s))
		}
	}
	return parts
}

// GraphQLComposedTypeName builds Tag_Update_Payload style names from model id + suffix (e.g. "Create_Payload", "List_Aggregate", "RawModel").
func GraphQLComposedTypeName(modelID string, suffix string) string {
	parts := modelIDSegments(modelID)
	parts = appendSuffixSegments(parts, suffix)
	for i, p := range parts {
		parts[i] = englishTitleWord(p)
	}
	return strings.Join(parts, "_")
}

// PascalFromAnyModelID builds GraphQL PascalCase type names from either canonical snake or legacy camel.
func PascalFromAnyModelID(modelID string) string {
	if modelID == "" {
		return ""
	}
	if strings.Contains(modelID, "_") {
		return PascalFromCanonical(modelID)
	}
	segs := camelcase.Split(modelID)
	var b strings.Builder
	for _, s := range segs {
		if s == "" {
			continue
		}
		low := strings.ToLower(s)
		b.WriteString(englishTitleWord(low))
	}
	return b.String()
}

// ListGraphQLTypeName returns the PascalCase list wrapper type (e.g. FoodCategoryList).
func ListGraphQLTypeName(modelID string) string {
	return PascalFromAnyModelID(modelID) + "List"
}

// GraphQLTypeNameForFilterArg is the PascalCase name passed to BuildFilterArgument (list query / count).
// It matches ListGraphQLTypeName for canonical and legacy ids.
func GraphQLTypeNameForFilterArg(modelID string) string {
	return ListGraphQLTypeName(modelID)
}

// WhereFilterConditionGraphQLTypeName returns a unique GraphQL input type for per-field where filters.
// Model and field are separate segments so loan.installment_amount and loan_installment.amount do not collide.
func WhereFilterConditionGraphQLTypeName(modelName, fieldIdentifier string) string {
	m := strings.TrimSpace(modelName)
	f := strings.TrimSpace(fieldIdentifier)
	return strings.ToUpper(m + "__FIELD__" + f + "__COMMON_FILTER_CONDITION")
}

// ModelIDMatchesGraphQLField reports whether a stored model id matches the lowerCamel resource id used in GraphQL field names.
func ModelIDMatchesGraphQLField(storedModelID, graphqlLowerCamel string) bool {
	if storedModelID == "" || graphqlLowerCamel == "" {
		return false
	}
	if storedModelID == graphqlLowerCamel {
		return true
	}
	return CamelFromAny(storedModelID) == graphqlLowerCamel
}

const systemRelationFieldAs = "_as_"

// CanonicalSystemRelationFieldIdentifier maps stored schema field ids to the same keys used in
// document data and connect/disconnect: system_<canonical_model_id>_id or
// system_<canonical_model_id>_as_<known_as>_id. Legacy schema rows may use mixed casing
// (e.g. system_foodCategory_id); this normalizes to system_food_category_id.
func CanonicalSystemRelationFieldIdentifier(identifier string) string {
	const pfx, sfx = "system_", "_id"
	if !strings.HasPrefix(identifier, pfx) || !strings.HasSuffix(identifier, sfx) {
		return identifier
	}
	core := strings.TrimSuffix(strings.TrimPrefix(identifier, pfx), sfx)
	if core == "" {
		return identifier
	}
	if idx := strings.Index(core, systemRelationFieldAs); idx >= 0 {
		fromPart, toPart := core[:idx], core[idx+len(systemRelationFieldAs):]
		if fromPart == "" || toPart == "" {
			return identifier
		}
		a, errA := CanonicalizeModelName(fromPart)
		b, errB := CanonicalizeModelName(toPart)
		if errA != nil || errB != nil {
			return identifier
		}
		return pfx + a + systemRelationFieldAs + b + sfx
	}
	c, err := CanonicalizeModelName(core)
	if err != nil {
		return identifier
	}
	return pfx + c + sfx
}

// SyntheticSystemRelationFieldIdentifier builds system_<canonical_model>_id or
// system_<canonical_model>_as_<known_as>_id for schema-stored synthetic relation fields.
// Segments use PhysicalSQLTableName so they align with SQL FK columns. Do not use raw
// Connection.Model in fmt.Sprintf — lowerCamel or collapsed lowercase breaks SQLite column names.
func SyntheticSystemRelationFieldIdentifier(modelID, knownAs string) string {
	modelID = strings.TrimSpace(modelID)
	knownAs = strings.TrimSpace(knownAs)
	if modelID == "" {
		return ""
	}
	mSeg := PhysicalSQLTableName(modelID)
	if knownAs != "" {
		return fmt.Sprintf("system_%s_as_%s_id", mSeg, PhysicalSQLTableName(knownAs))
	}
	return fmt.Sprintf("system_%s_id", mSeg)
}

// RelationFilterGraphQLKey is the public GraphQL key for relation list filters and nested
// has_one relation fields. When known_as is set it is the field alias (e.g. owner); otherwise
// the canonical stored model id (snake_case, e.g. food_category). Query roots like
// foodCategoryList stay lowerCamel via SingularResourceName / MultipleResourceName.
func RelationFilterGraphQLKey(modelName, knownAs string) string {
	if strings.TrimSpace(knownAs) != "" {
		return knownAs
	}
	return PhysicalSQLTableName(modelName)
}

// RelationNestedListGraphQLKey is the nested has_many field on a parent type
// (e.g. food_category_list, chef_list). Fully snake_case — only root query/mutation
// operation names use lowerCamel (MultipleResourceName → foodCategoryList).
func RelationNestedListGraphQLKey(modelName, knownAs string) string {
	base := RelationFilterGraphQLKey(modelName, knownAs)
	if base == "" {
		return ""
	}
	return base + "_list"
}
