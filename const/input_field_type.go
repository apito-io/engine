package _const

// int, bool all has to be a golang enum

const (
	IntInput      string = "int"
	BoolInput     string = "bool"
	DoubleInput   string = "double"
	StringInput   string = "string"
	GeoInput      string = "geo"
	ObjectInput   string = "object"
	RepeatedInput string = "repeated"
)

const (
	TextField      string = "text"
	MediaField     string = "media"
	MultilineField string = "multiline"
	DateField      string = "date"
	NumberField    string = "number"
	BooleanField   string = "boolean"
	ListField      string = "list"
	GeoField       string = "geo"
	ObjectField    string = "object"
	RepeatedField  string = "repeated"
)

var MapFieldInputType = map[string]string{
	TextField:      StringInput,
	MultilineField: StringInput,
	DateField:      StringInput,
	ListField:      StringInput,
	BooleanField:   BoolInput,
	MediaField:     StringInput,
	NumberField:    IntInput,
	GeoField:       GeoInput,
	ObjectField:    ObjectInput,
	RepeatedField:  RepeatedInput,
	"relation":  "relation",
}

// #todo add better error handling if the type doest not exits or unmatched
func GetInputTypebyFieldType(inputType string) string {
	if val, ok := MapFieldInputType[inputType]; ok {
		return val
	}
	return StringInput // string is the default type
}
