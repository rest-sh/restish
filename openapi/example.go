package openapi

import (
	"log"
	"maps"
	"strings"

	"github.com/lucasjones/reggen"
	"github.com/pb33f/libopenapi/datamodel/high/base"
)

// GenExample creates a dummy example from a given schema.
func GenExample(schema *base.Schema, mode schemaMode) any {
	example, err := genExampleInternal(schema, mode, map[[32]byte]bool{})
	if err != nil {
		log.Fatal(err)
	}
	return example
}

func genExampleInternal(s *base.Schema, mode schemaMode, known map[[32]byte]bool) (any, error) {
	inferType(s)

	// TODO: handle  not
	if len(s.OneOf) > 0 {
		return genExampleInternal(s.OneOf[0].Schema(), mode, known)
	}

	if len(s.AnyOf) > 0 {
		return genExampleInternal(s.AnyOf[0].Schema(), mode, known)
	}

	if len(s.AllOf) > 0 {
		result := map[string]any{}
		for _, proxy := range s.AllOf {
			tmp, err := genExampleInternal(proxy.Schema(), mode, known)
			if err != nil {
				return nil, err
			}
			if m, ok := tmp.(map[string]any); ok {
				maps.Copy(result, m)
			}
		}
		return result, nil
	}

	if s.Example != nil {
		return decodeYAML(s.Example)
	}

	if len(s.Examples) > 0 {
		return decodeYAML(s.Examples[0])
	}

	if s.Default != nil {
		return decodeYAML(s.Default)
	}

	if s.Minimum != nil {
		if s.ExclusiveMinimum != nil && s.ExclusiveMinimum.IsA() && s.ExclusiveMinimum.A {
			return *s.Minimum + 1, nil
		}
		return *s.Minimum, nil
	} else if s.ExclusiveMinimum != nil && (s.ExclusiveMinimum.IsB()) {
		return s.ExclusiveMinimum.B + 1, nil
	}

	if s.Maximum != nil {
		if s.ExclusiveMaximum != nil && s.ExclusiveMaximum.IsA() && s.ExclusiveMaximum.A {
			return *s.Maximum - 1, nil
		}
		return *s.Maximum, nil
	} else if s.ExclusiveMaximum != nil && s.ExclusiveMaximum.IsB() {
		return s.ExclusiveMaximum.B - 1, nil
	}

	if s.MultipleOf != nil && *s.MultipleOf != 0 {
		return *s.MultipleOf, nil
	}

	if len(s.Enum) > 0 {
		return decodeYAML(s.Enum[0])
	}

	if s.Pattern != "" {
		if g, err := reggen.NewGenerator(s.Pattern); err == nil {
			// We need stable/reproducible outputs, so use a constant seed.
			g.SetSeed(1589525091)
			return g.Generate(3), nil
		}
	}

	switch s.Format {
	case "date":
		return "2020-05-14", nil
	case "time":
		return "23:44:51-07:00", nil
	case "date-time":
		return "2020-05-14T23:44:51-07:00", nil
	case "duration":
		return "P30S", nil
	case "email", "idn-email":
		return "user@example.com", nil
	case "hostname", "idn-hostname":
		return "example.com", nil
	case "ipv4":
		return "192.0.2.1", nil
	case "ipv6":
		return "2001:db8::1", nil
	case "uuid":
		return "3e4666bf-d5e5-4aa7-b8ce-cefe41c7568a", nil
	case "uri", "iri":
		return "https://example.com/", nil
	case "uri-reference", "iri-reference":
		return "/example", nil
	case "uri-template":
		return "https://example.com/{id}", nil
	case "json-pointer":
		return "/example/0/id", nil
	case "relative-json-pointer":
		return "0/id", nil
	case "regex":
		return "ab+c", nil
	case "password":
		return "********", nil
	}

	typ := ""
	for _, t := range s.Type {
		// Find the first non-null type and use that for now.
		if t != "null" {
			typ = t
			break
		}
	}

	switch typ {
	case "boolean":
		return true, nil
	case "integer":
		return 1, nil
	case "number":
		return 1.0, nil
	case "string":
		if s.MinLength != nil && *s.MinLength > 6 {
			sb := strings.Builder{}
			for i := int64(0); i < *s.MinLength; i++ {
				sb.WriteRune('s')
			}
			return sb.String(), nil
		}

		if s.MaxLength != nil && *s.MaxLength < 6 {
			sb := strings.Builder{}
			for i := int64(0); i < *s.MaxLength; i++ {
				sb.WriteRune('s')
			}
			return sb.String(), nil
		}

		return "string", nil
	case "array":
		if s.Items != nil && s.Items.IsA() {
			items := s.Items.A.Schema()
			simple := isSimpleSchema(items)
			hash := items.GoLow().Hash()
			if simple || !known[hash] {
				known[hash] = true
				item, err := genExampleInternal(items, mode, known)
				if err != nil {
					return nil, err
				}
				known[hash] = false

				count := 1
				if s.MinItems != nil && *s.MinItems > 0 {
					count = int(*s.MinItems)
				}

				value := make([]any, 0, count)
				for i := 0; i < count; i++ {
					value = append(value, item)
				}
				return value, nil
			}

			return []any{nil}, nil
		}
		return "[<any>]", nil
	case "object":
		value := map[string]any{}

		// Special case: object with nothing defined
		if s.Properties != nil && s.Properties.Len() == 0 && s.AdditionalProperties == nil {
			return value, nil
		}

		for name, proxy := range s.Properties.FromOldest() {
			prop := proxy.Schema()
			if prop == nil {
				continue
			}
			if derefOrDefault(prop.ReadOnly) && mode == modeWrite {
				continue
			} else if derefOrDefault(prop.WriteOnly) && mode == modeRead {
				continue
			}

			simple := isSimpleSchema(prop)
			hash := prop.GoLow().Hash()
			if simple || !known[hash] {
				known[hash] = true
				var err error
				value[name], err = genExampleInternal(prop, mode, known)
				if err != nil {
					return nil, err
				}
				known[hash] = false
			} else {
				value[name] = nil
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
						var err error
						value["<any>"], err = genExampleInternal(addl, mode, known)
						if err != nil {
							return nil, err
						}
						known[hash] = false
					} else {
						value["<any>"] = nil
					}
				}
				if ap.IsB() && ap.B {
					value["<any>"] = nil
				}
			}
		}

		return value, nil
	}

	return nil, nil
}
