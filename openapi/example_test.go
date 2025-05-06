package openapi

import (
	"context"
	"testing"

	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/datamodel/low"
	lowbase "github.com/pb33f/libopenapi/datamodel/low/base"
	"github.com/pb33f/libopenapi/index"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var exampleTests = []struct {
	name          string
	mode          schemaMode
	in            string
	inSpecVersion string
	out           any
}{
	{
		name:          "boolean",
		in:            `{type: boolean}`,
		inSpecVersion: datamodel.OAS3,
		out:           true,
	},
	{
		name:          "example",
		in:            `{type: number, example: 5}`,
		inSpecVersion: datamodel.OAS3,
		out:           5,
	},
	{
		name:          "examples",
		in:            `{type: number, examples: [5]}`,
		inSpecVersion: datamodel.OAS3,
		out:           5,
	},
	{
		name:          "guess-array",
		in:            `items: {type: string}`,
		inSpecVersion: datamodel.OAS3,
		out:           []any{"string"},
	},
	{
		name:          "guess-object",
		in:            `additionalProperties: true`,
		inSpecVersion: datamodel.OAS3,
		out:           map[string]any{"<any>": nil},
	},
	{
		name:          "min",
		in:            `{type: number, minimum: 5}`,
		inSpecVersion: datamodel.OAS3,
		out:           5,
	},
	{
		name:          "exclusive-min",
		in:            `{type: number, minimum: 5, exclusiveMinimum: true}`,
		inSpecVersion: datamodel.OAS3,
		out:           6,
	},
	{
		name:          "exclusive-min-31",
		in:            `{type: number, exclusiveMinimum: 5}`,
		inSpecVersion: datamodel.OAS31,
		out:           6,
	},
	{
		name:          "max",
		in:            `{type: number, maximum: 5}`,
		inSpecVersion: datamodel.OAS3,
		out:           5,
	},
	{
		name:          "exclusive-max",
		in:            `{type: number, maximum: 5, exclusiveMaximum: true}`,
		inSpecVersion: datamodel.OAS3,
		out:           4,
	},
	{
		name:          "exclusive-max-31",
		in:            `{type: number, exclusiveMaximum: 5}`,
		inSpecVersion: datamodel.OAS31,
		out:           4,
	},
	{
		name:          "multiple-of",
		in:            `{type: number, multipleOf: 5}`,
		inSpecVersion: datamodel.OAS3,
		out:           5,
	},
	{
		name:          "default-scalar",
		in:            `{type: number, default: 5.0}`,
		inSpecVersion: datamodel.OAS3,
		out:           5,
	},
	{
		name:          "default-object",
		in:            `{type: object, default: {foo: hello}}`,
		inSpecVersion: datamodel.OAS3,
		out:           map[string]any{"foo": "hello"},
	},
	{
		name:          "string-format-date",
		in:            `{type: string, format: date}`,
		inSpecVersion: datamodel.OAS3,
		out:           "2020-05-14",
	},
	{
		name:          "string-format-time",
		in:            `{type: string, format: time}`,
		inSpecVersion: datamodel.OAS3,
		out:           "23:44:51-07:00",
	},
	{
		name:          "string-format-date-time",
		in:            `{type: string, format: date-time}`,
		inSpecVersion: datamodel.OAS3,
		out:           "2020-05-14T23:44:51-07:00",
	},
	{
		name:          "string-format-duration",
		in:            `{type: string, format: duration}`,
		inSpecVersion: datamodel.OAS3,
		out:           "P30S",
	},
	{
		name:          "string-format-email",
		in:            `{type: string, format: email}`,
		inSpecVersion: datamodel.OAS3,
		out:           "user@example.com",
	},
	{
		name:          "string-format-hostname",
		in:            `{type: string, format: hostname}`,
		inSpecVersion: datamodel.OAS3,
		out:           "example.com",
	},
	{
		name:          "string-format-ipv4",
		in:            `{type: string, format: ipv4}`,
		inSpecVersion: datamodel.OAS3,
		out:           "192.0.2.1",
	},
	{
		name:          "string-format-ipv6",
		in:            `{type: string, format: ipv6}`,
		inSpecVersion: datamodel.OAS3,
		out:           "2001:db8::1",
	},
	{
		name:          "string-format-uuid",
		in:            `{type: string, format: uuid}`,
		inSpecVersion: datamodel.OAS3,
		out:           "3e4666bf-d5e5-4aa7-b8ce-cefe41c7568a",
	},
	{
		name:          "string-format-uri",
		in:            `{type: string, format: uri}`,
		inSpecVersion: datamodel.OAS3,
		out:           "https://example.com/",
	},
	{
		name:          "string-format-uri-ref",
		in:            `{type: string, format: uri-reference}`,
		inSpecVersion: datamodel.OAS3,
		out:           "/example",
	},
	{
		name:          "string-format-uri-template",
		in:            `{type: string, format: uri-template}`,
		inSpecVersion: datamodel.OAS3,
		out:           "https://example.com/{id}",
	},
	{
		name:          "string-format-json-pointer",
		in:            `{type: string, format: json-pointer}`,
		inSpecVersion: datamodel.OAS3,
		out:           "/example/0/id",
	},
	{
		name:          "string-format-rel-json-pointer",
		in:            `{type: string, format: relative-json-pointer}`,
		inSpecVersion: datamodel.OAS3,
		out:           "0/id",
	},
	{
		name:          "string-format-regex",
		in:            `{type: string, format: regex}`,
		inSpecVersion: datamodel.OAS3,
		out:           "ab+c",
	},
	{
		name:          "string-format-password",
		in:            `{type: string, format: password}`,
		inSpecVersion: datamodel.OAS3,
		out:           "********",
	},
	{
		name:          "string-pattern",
		in:            `{type: string, pattern: "^[a-z]+$"}`,
		inSpecVersion: datamodel.OAS3,
		out:           "qne",
	},
	{
		name:          "string-min-length",
		in:            `{type: string, minLength: 10}`,
		inSpecVersion: datamodel.OAS3,
		out:           "ssssssssss",
	},
	{
		name:          "string-max-length",
		in:            `{type: string, maxLength: 3}`,
		inSpecVersion: datamodel.OAS3,
		out:           "sss",
	},
	{
		name:          "string-enum",
		in:            `{type: string, enum: [one, two]}`,
		inSpecVersion: datamodel.OAS3,
		out:           "one",
	},
	{
		name:          "empty-array",
		in:            `{type: array}`,
		inSpecVersion: datamodel.OAS3,
		out:           "[<any>]",
	},
	{
		name:          "array",
		in:            `{type: array, items: {type: number}}`,
		inSpecVersion: datamodel.OAS3,
		out:           []any{1.0},
	},
	{
		name:          "array-min-items",
		in:            `{type: array, items: {type: number}, minItems: 2}`,
		inSpecVersion: datamodel.OAS3,
		out:           []any{1.0, 1.0},
	},
	{
		name:          "object-empty",
		in:            `{type: object}`,
		inSpecVersion: datamodel.OAS3,
		out:           map[string]any{},
	},
	{
		name:          "object-prop-null",
		in:            `{type: object, properties: {foo: null}}`,
		inSpecVersion: datamodel.OAS3,
		out:           map[string]any{"foo": nil},
	},
	{
		name:          "object",
		in:            `{type: object, properties: {foo: {type: string}, bar: {type: integer}}, required: [foo]}`,
		inSpecVersion: datamodel.OAS3,
		out: map[string]any{
			"foo": "string",
			"bar": 1,
		},
	},
	{
		name:          "object-read-only",
		mode:          modeRead,
		in:            `{type: object, properties: {foo: {type: string, readOnly: true}, bar: {type: string, writeOnly: true}}}`,
		inSpecVersion: datamodel.OAS3,
		out:           map[string]any{"foo": "string"},
	},
	{
		name:          "object-write-only",
		mode:          modeWrite,
		in:            `{type: object, properties: {foo: {type: string, readOnly: true}, bar: {type: string, writeOnly: true}}}`,
		inSpecVersion: datamodel.OAS3,

		out: map[string]any{"bar": "string"},
	},
	{
		name:          "object-additional-props-bool",
		in:            `{type: object, additionalProperties: true}`,
		inSpecVersion: datamodel.OAS3,
		out:           map[string]any{"<any>": nil},
	},
	{
		name:          "object-additional-props-scehma",
		in:            `{type: object, additionalProperties: {type: string}}`,
		inSpecVersion: datamodel.OAS3,
		out:           map[string]any{"<any>": "string"},
	},
	{
		name:          "all-of",
		in:            `{allOf: [{type: object, properties: {a: {type: string}}}, {type: object, properties: {foo: {type: string}, bar: {type: number, description: desc}}}]}`,
		inSpecVersion: datamodel.OAS3,
		out: map[string]any{
			"a":   "string",
			"bar": 1.0,
			"foo": "string",
		},
	},
	{
		name:          "one-of",
		in:            `{oneOf: [{type: boolean}, {type: object, properties: {foo: {type: string}, bar: {type: number, description: desc}}}]}`,
		inSpecVersion: datamodel.OAS3,
		out:           true,
	},
	{
		name:          "any-of",
		in:            `{anyOf: [{type: boolean}, {type: object, properties: {foo: {type: string}, bar: {type: number, description: desc}}}]}`,
		inSpecVersion: datamodel.OAS3,
		out:           true,
	},
	{
		name:          "recusive-prop",
		in:            `{type: object, properties: {person: {type: object, properties: {friend: {$ref: "#/properties/person"}}}}}`,
		inSpecVersion: datamodel.OAS3,
		out: map[string]any{
			"person": map[string]any{
				"friend": nil,
			},
		},
	},
	{
		name:          "recusive-array",
		in:            `{type: object, properties: {person: {type: object, properties: {friend: {type: array, items: {$ref: "#/properties/person"}}}}}}`,
		inSpecVersion: datamodel.OAS3,
		out: map[string]any{
			"person": map[string]any{
				"friend": []any{nil},
			},
		},
	},
	{
		name:          "recusive-additional-props",
		in:            `{type: object, properties: {person: {type: object, properties: {friend: {type: object, additionalProperties: {$ref: "#/properties/person"}}}}}}`,
		inSpecVersion: datamodel.OAS3,
		out: map[string]any{
			"person": map[string]any{
				"friend": map[string]any{
					"<any>": nil,
				},
			},
		},
	},
}

func TestExample(t *testing.T) {
	for _, example := range exampleTests {
		t.Run(example.name, func(t *testing.T) {
			var rootNode yaml.Node
			var ls lowbase.Schema

			require.NoError(t, yaml.Unmarshal([]byte(example.in), &rootNode))
			require.NoError(t, low.BuildModel(rootNode.Content[0], &ls))
			specIndex := index.NewSpecIndex(&rootNode)

			if example.inSpecVersion == datamodel.OAS3 {
				inf, err := datamodel.ExtractSpecInfo([]byte(`openapi: 3.0.1`))
				require.NoError(t, err)
				specIndex.GetConfig().SpecInfo = inf
			} else {
				inf, err := datamodel.ExtractSpecInfo([]byte(`openapi: 3.1.0`))
				require.NoError(t, err)
				specIndex.GetConfig().SpecInfo = inf
			}
			require.NoError(t, ls.Build(context.Background(), rootNode.Content[0], specIndex))

			// spew.Dump(ls)

			s := base.NewSchema(&ls)
			assert.EqualValues(t, example.out, GenExample(s, example.mode))
		})
	}
}
