package openapi

import (
	"bytes"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"gopkg.in/yaml.v3"
)

type schemaMode int

const (
	modeRead schemaMode = iota
	modeWrite
)

// inferType fixes missing type if it is missing & can be inferred
func inferType(s *base.Schema) {
	if len(s.Type) == 0 {
		if s.Items != nil {
			s.Type = []string{"array"}
		}
		if (s.Properties != nil && s.Properties.Len() > 0) || s.AdditionalProperties != nil {
			s.Type = []string{"object"}
		}
	}
}

// isSimpleSchema returns whether this schema is a scalar or array as these
// can't be circular references. Objects result in `false` and that triggers
// circular ref checks.
func isSimpleSchema(s *base.Schema) bool {
	if len(s.Type) == 0 {
		return true
	}

	return s.Type[0] != "object"
}

func renderSchema(s *base.Schema, indent string, mode schemaMode) string {
	return renderSchemaInternal(s, indent, mode, map[[32]byte]bool{})
}

func derefOrDefault[T any](v *T) T {
	if v != nil {
		return *v
	}
	var d T
	return d
}

func renderSchemaInternal(s *base.Schema, indent string, mode schemaMode, known map[[32]byte]bool) string {
	doc := s.Title
	if doc == "" {
		doc = s.Description
	}

	inferType(s)

	// TODO: handle not
	for _, of := range []struct {
		label   string
		schemas []*base.SchemaProxy
	}{
		{label: "allOf", schemas: s.AllOf},
		{label: "oneOf", schemas: s.OneOf},
		{label: "anyOf", schemas: s.AnyOf},
	} {
		if len(of.schemas) > 0 {
			out := of.label + "{\n"
			for _, possible := range of.schemas {
				sch := possible.Schema()
				simple := isSimpleSchema(sch)
				hash := sch.GoLow().Hash()
				if simple || !known[hash] {
					known[hash] = true
					out += indent + "  " + renderSchemaInternal(possible.Schema(), indent+"  ", mode, known) + "\n"
					known[hash] = false
					continue
				}
				out += indent + "  <recursive ref>\n"
			}
			return out + indent + "}"
		}
	}

	// TODO: list type alternatives somehow?
	typ := ""
	for _, t := range s.Type {
		// Find the first non-null type and use that for now.
		if t != "null" {
			typ = t
			break
		}
	}

	switch typ {
	case "boolean", "integer", "number", "string":
		tags := []string{}

		// TODO: handle more validators
		if s.Nullable != nil && *s.Nullable {
			tags = append(tags, "nullable:true")
		}

		if s.Minimum != nil {
			key := "min"
			if s.ExclusiveMinimum != nil && s.ExclusiveMinimum.IsA() && s.ExclusiveMinimum.A {
				key = "exclusiveMin"
			}
			tags = append(tags, fmt.Sprintf("%s:%g", key, *s.Minimum))
		} else if s.ExclusiveMinimum != nil && s.ExclusiveMinimum.IsB() {
			tags = append(tags, fmt.Sprintf("exclusiveMin:%g", s.ExclusiveMinimum.B))
		}

		if s.Maximum != nil {
			key := "max"
			if s.ExclusiveMaximum != nil && s.ExclusiveMaximum.IsA() && s.ExclusiveMaximum.A {
				key = "exclusiveMax"
			}
			tags = append(tags, fmt.Sprintf("%s:%g", key, *s.Maximum))
		} else if s.ExclusiveMaximum != nil && s.ExclusiveMaximum.IsB() {
			tags = append(tags, fmt.Sprintf("exclusiveMax:%g", s.ExclusiveMaximum.B))
		}

		if s.MultipleOf != nil && *s.MultipleOf != 0 {
			tags = append(tags, fmt.Sprintf("multiple:%g", *s.MultipleOf))
		}

		if s.Default != nil {
			def, err := yaml.Marshal(s.Default)
			if err != nil {
				log.Fatal(err)
			}
			tags = append(tags, fmt.Sprintf("default:%v", string(bytes.TrimSpace(def))))
		}

		if s.Format != "" {
			tags = append(tags, fmt.Sprintf("format:%v", s.Format))
		}

		if s.Pattern != "" {
			tags = append(tags, fmt.Sprintf("pattern:%s", s.Pattern))
		}

		if s.MinLength != nil && *s.MinLength != 0 {
			tags = append(tags, fmt.Sprintf("minLen:%d", *s.MinLength))
		}

		if s.MaxLength != nil && *s.MaxLength != 0 {
			tags = append(tags, fmt.Sprintf("maxLen:%d", *s.MaxLength))
		}

		if len(s.Enum) > 0 {
			enums := []string{}
			for _, e := range s.Enum {
				ev, err := yaml.Marshal(e)
				if err != nil {
					log.Fatal(err)
				}
				enums = append(enums, fmt.Sprintf("%v", string(bytes.TrimSpace(ev))))
			}

			tags = append(tags, fmt.Sprintf("enum:%s", strings.Join(enums, ",")))
		}

		tagStr := ""
		if len(tags) > 0 {
			tagStr = " " + strings.Join(tags, " ")
		}

		if doc != "" {
			doc = " " + doc
		}
		return fmt.Sprintf("(%s%s)%s", strings.Join(s.Type, "|"), tagStr, doc)
	case "array":
		if s.Items != nil && s.Items.IsA() {
			items := s.Items.A.Schema()
			simple := isSimpleSchema(items)
			hash := items.GoLow().Hash()
			if simple || !known[hash] {
				known[hash] = true
				arr := "[\n  " + indent + renderSchemaInternal(items, indent+"  ", mode, known) + "\n" + indent + "]"
				known[hash] = false
				return arr
			}

			return "[<recursive ref>]"
		}
		return "[<any>]"
	case "object":
		// Special case: object with nothing defined
		if (s.Properties == nil || s.Properties.Len() == 0) && s.AdditionalProperties == nil {
			return "(object)"
		}

		var obj strings.Builder
		obj.WriteString("{\n")

		keys := slices.Sorted(s.Properties.KeysFromOldest())

		for _, name := range keys {
			propVal := s.Properties.Value(name)
			prop := propVal.Schema()
			if prop == nil {
				if err := propVal.GetBuildError(); err != nil {
					log.Fatal(err)
				}
				continue
			}

			if derefOrDefault(prop.ReadOnly) && mode == modeWrite {
				continue
			} else if derefOrDefault(prop.WriteOnly) && mode == modeRead {
				continue
			}

			if slices.Contains(s.Required, name) {
				name += "*"
			}

			simple := isSimpleSchema(prop)
			hash := prop.GoLow().Hash()
			if simple || !known[hash] {
				known[hash] = true
				obj.WriteString(indent + "  " + name + ": " + renderSchemaInternal(prop, indent+"  ", mode, known) + "\n")
				known[hash] = false
			} else {
				obj.WriteString(indent + "  " + name + ": <rescurive ref>\n")
			}
		}

		if s.AdditionalProperties != nil {
			ap := s.AdditionalProperties
			if ap != nil {
				if ap.IsA() && ap.A != nil {
					addl := ap.A.Schema()
					simple := isSimpleSchema(addl)
					hash := addl.GoLow().Hash()
					if simple || !known[hash] {
						known[hash] = true
						obj.WriteString(indent + "  " + "<any>: " + renderSchemaInternal(addl, indent+"  ", mode, known) + "\n")
					} else {
						obj.WriteString(indent + "  <any>: <rescurive ref>\n")
					}
				}
				if ap.IsB() && ap.B {
					obj.WriteString(indent + "  <any>: <any>\n")
				}
			}
		}

		obj.WriteString(indent + "}")
		return obj.String()
	}

	return "<any>"
}
