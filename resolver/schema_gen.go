package resolver

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/apito-io/buffers/protobuff"
	"github.com/jinzhu/inflection"
)

type SchemGenDirectories struct {
	ProjectId       string // used in resolver.go generation
	ProjectRoot     string
	EngineDir       string
	CacheDir        string
	ProjectCacheDir string
	IsSubSchema     bool // used in schema generator to identify if additional fields need to be generated or not
}

func PrepareDirectories(ctx context.Context, projectId string) *SchemGenDirectories {
	// Load Local Plugin
	_workDir, _ := os.Getwd()
	projectRoot := filepath.Dir(_workDir)

	return &SchemGenDirectories{
		ProjectId:       projectId,
		ProjectRoot:     projectRoot,
		EngineDir:       filepath.Join(projectRoot, "engine"),
		CacheDir:        filepath.Join(projectRoot, "engine", "cache"),
		ProjectCacheDir: filepath.Join(projectRoot, "engine", "cache", "schema", projectId),
	}
}

func (s *GraphQLServer) genSchema(ctx context.Context, _dir *SchemGenDirectories, _schema *protobuff.ProjectSchema) (*protobuff.ProjectSchema, error) {
	_subSchema := protobuff.ProjectSchema{}

	var _template string
	if _dir.IsSubSchema {
		_template = filepath.Join(_dir.EngineDir, "gqlgen/template/sub-schema-gen-template.gohtml")
	} else {
		_template = filepath.Join(_dir.EngineDir, "gqlgen/template/schema-gen-base-template.gohtml")
	}

	base := path.Base(_template)
	t, err := template.New(base).Funcs(template.FuncMap{
		"query_name_formatter": func(_val string) string {
			singular := inflection.Singular(_val)
			plural := inflection.Plural(_val)
			if singular == plural {
				return fmt.Sprintf(`get%s`, strings.Title(singular))
			}
			return inflection.Singular(_val)
		},
		"singular": func(_val string) string {
			return inflection.Singular(_val)
		},
		"singular_title": func(_val string) string {
			return strings.Title(inflection.Singular(_val))
		},
		"plural": func(_val string) string {
			return inflection.Plural(_val)
		},
		"title": func(_val string) string {
			return strings.Title(_val)
		},
		"input_args_format": func(_parent string, _val *protobuff.FieldInfo, entry bool) string {
			var _type string
			// input type special filter
			switch _val.InputType {
			case "string":
				switch _val.FieldType {
				case "list":
					if !_val.Validation.IsMultiChoice && len(_val.Validation.FixedListElements) > 0 { // for dropdown
						_type = "DROPDOWN_ARGS"
					} else {
						_type = "LIST_ARGS"
					}
					break
				case "date":
					_type = "DATE_ARGS"
					break
				case "multiline":
					_type = "MULTILINE_ARGS"
					break
				default:
					_type = "DEFAULT_ARGS"
				}
				break
			case "int":
				_type = "INTEGER_ARGS"
				break
			case "double":
				_type = "DOUBLE_ARGS"
				break
			case "bool":
				_type = "BOOLEAN_ARGS"
				break
			case "geo":
				/*		fields["geo_near"] = &graphql.InputObjectFieldConfig{
						Type: graphql.NewInputObject(graphql.InputObjectConfig{
							Name: strings.ToUpper(name) + "_GEO_NEAR_INPUT",
							Fields: graphql.InputObjectConfigFieldMap{
								"lat": &graphql.InputObjectFieldConfig{
									Type: graphql.Float,
								},
								"lon": &graphql.InputObjectFieldConfig{
									Type: graphql.Float,
								},
								"nth": &graphql.InputObjectFieldConfig{
									Description: " n closest coordinates to a reference point, and return the documents with the nearby locations. The default for n is 100, which means 100 documents are returned at most, the closest matches first. Default is 3",
									Type: graphql.Int,
								},
							},
						}),
					}*/
				_type = "GEO_ARGS"
				break
			case "repeated", "object":
				_type = fmt.Sprintf(`%sWhereArgs`, strings.Title(_parent))
			}
			return _type
		},
		"field_formatter": func(_format string, _parent string, _val *protobuff.FieldInfo, entry bool, _skipValidation bool) string {
			var _type string
			switch _val.InputType {
			case "string", "int", "multiline":
				_type = strings.Title(_val.InputType)
				switch _val.FieldType {
				case "multiline":
					switch _format {
					case "input":
						_type = "MULTILINE_INPUT_TYPE"
					case "type":
						_type = "MULTILINE_FIELD_TYPE"
					}
				case "media":
					switch _format {
					case "input":
						_type = "MEDIA_INPUT_TYPE"
					case "type":
						_type = "MEDIA_FIELD_TYPE"
					}
				}
			case "bool":
				_type = "Boolean"
			case "geo":
				if _format == "input" {
					_type = "GEO_INPUT"
				} else {
					_type = "GEO_FIELDS"
				}
			case "repeated":
				_type = fmt.Sprintf(`%s_%s`, strings.Title(_parent), strings.Title(_val.Identifier))
				if entry {
					_subSchema.Models = append(_subSchema.Models, &protobuff.ModelType{
						Name:   _type,
						Fields: _val.SubFieldInfo,
					})
				}
				if _format == "input" {
					_type = fmt.Sprintf(`[%s_Input]`, _type)
				} else {
					_type = fmt.Sprintf(`[%s]`, _type)
				}
			case "object":
				_type = fmt.Sprintf(`%s_%s`, strings.Title(_parent), strings.Title(_val.Identifier))
				if entry {
					_subSchema.Models = append(_subSchema.Models, &protobuff.ModelType{
						Name:   _type,
						Fields: _val.SubFieldInfo,
					})
				}
				if _format == "input" {
					_type = fmt.Sprintf(`%s_Input`, _type)
				} else {
					_type = fmt.Sprintf(`%s`, _type)
				}
			case "double":
				_type = "Float"
			case "boolean":
				_type = "Boolean"
			default:
				_type = ""
			}

			if !_skipValidation && _val.Validation != nil && _val.Validation.Required {
				return fmt.Sprintf(`%s!`, _type)
			}

			return _type
		},
		"connection_format": func(_val *protobuff.ConnectionType) string {
			var _type string
			switch _val.Relation {
			case "has_many":
				_type = fmt.Sprintf(`[%s]`, strings.Title(_val.Model))
			case "has_one":
				_type = fmt.Sprintf(`%s`, strings.Title(_val.Model))
			default:
				_type = ""
			}
			return _type
		},
		"connection_known_as_format": func(_val *protobuff.ConnectionType) string {
			var _type string
			switch _val.Relation {
			case "has_many":
				_type = fmt.Sprintf(`%s`, inflection.Plural(_val.Model))
			case "has_one":
				_type = fmt.Sprintf(`%s`, inflection.Singular(_val.Model))
			default:
				_type = fmt.Sprintf(`%s`, _val.Model)
			}
			// known as overwrite all the model name
			if _val.KnownAs != "" {
				_type = fmt.Sprintf(`%s`, _val.KnownAs)
			}
			return _type
		},
		"connection_input_format": func(_val *protobuff.ConnectionType) string {
			var _model string
			if _val.KnownAs != "" {
				_model = _val.KnownAs
			} else {
				_model = _val.Model
			}
			switch _val.Relation {
			case "has_many":
				return fmt.Sprintf(`%s_ids: [String]`, strings.ToLower(_model))
			case "has_one":
				return fmt.Sprintf(`%s_id: String`, strings.ToLower(_model))
			default:
				return ""
			}
		},
	}).ParseFiles(_template)
	if err != nil {
		return nil, err
	}

	var _schemaFile string
	if _dir.IsSubSchema {
		_schemaFile = filepath.Join(_dir.ProjectCacheDir, `graph/sub-schema.graphql`)
	} else {
		_schemaFile = filepath.Join(_dir.ProjectCacheDir, `graph/schema.graphql`)
	}

	f, err := os.Create(_schemaFile)
	if err != nil {
		var pathError *fs.PathError
		if errors.As(err, &pathError) {
			if err = os.MkdirAll(filepath.Join(_dir.ProjectCacheDir, "graph"), 0770); err != nil {
				return nil, err
			} else {
				f, err = os.Create(_schemaFile)
				if err != nil {
					return nil, err
				}
			}
		} else {
			return nil, err
		}
	}
	defer f.Close()

	// standard output to print merged data
	err = t.Execute(f, _schema)
	if err != nil {
		return nil, err
	}

	if len(_subSchema.Models) > 0 {
		//_o := strings.Split(_output, "/")
		//_output = fmt.Sprintf(`%s/input`, strings.Join(_o[0:len(_o)-1], "/"))
		_dir.IsSubSchema = true
		return s.genSchema(ctx, _dir, &_subSchema)
	}

	return &_subSchema, nil
}
