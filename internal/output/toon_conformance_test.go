package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

type toonEncodeFixture struct {
	Tests []toonEncodeCase `json:"tests"`
}

type toonEncodeCase struct {
	Name        string          `json:"name"`
	Input       json.RawMessage `json:"input"`
	Expected    string          `json:"expected"`
	Options     map[string]any  `json:"options"`
	ShouldError bool            `json:"shouldError"`
}

func TestTOONOfficialEncodeFixtures(t *testing.T) {
	files, err := filepath.Glob("testdata/toon/encode/*.json")
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no TOON encode fixtures found")
	}

	for _, file := range files {
		file := file
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fixture toonEncodeFixture
			if err := json.Unmarshal(raw, &fixture); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			for _, tc := range fixture.Tests {
				tc := tc
				t.Run(tc.Name, func(t *testing.T) {
					if tc.ShouldError {
						t.Skip("Restish TOON currently implements encoder success cases only")
					}
					if !toonFixtureUsesDefaultOptions(tc.Options) {
						t.Skip("Restish TOON currently implements the default delimiter, indentation, and key folding profile")
					}
					input := decodeOrderedJSONFixtureValue(t, tc.Input)
					if got := encodeTOONDocument(input); got != tc.Expected {
						t.Fatalf("official fixture mismatch:\n got: %q\nwant: %q", got, tc.Expected)
					}
				})
			}
		})
	}
}

func toonFixtureUsesDefaultOptions(options map[string]any) bool {
	for key, value := range options {
		switch key {
		case "delimiter":
			if value != "," {
				return false
			}
		case "indent":
			if value != float64(2) {
				return false
			}
		case "keyFolding":
			if value != "off" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func decodeOrderedJSONFixtureValue(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	value, err := decodeOrderedJSONValue(dec)
	if err != nil {
		t.Fatalf("decode ordered JSON fixture value: %v", err)
	}
	return value
}

func decodeOrderedJSONValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok := tok.(type) {
	case json.Delim:
		switch tok {
		case '{':
			var obj toonObject
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyTok.(string)
				if !ok {
					return nil, fmt.Errorf("object key token is %T", keyTok)
				}
				value, err := decodeOrderedJSONValue(dec)
				if err != nil {
					return nil, err
				}
				obj = append(obj, toonField{key: key, value: value})
			}
			_, err := dec.Token()
			return obj, err
		case '[':
			var arr []any
			for dec.More() {
				value, err := decodeOrderedJSONValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, value)
			}
			_, err := dec.Token()
			return arr, err
		default:
			return nil, fmt.Errorf("unexpected delimiter %q", tok)
		}
	case string, bool, nil, json.Number:
		return tok, nil
	default:
		return nil, fmt.Errorf("unexpected token %T", tok)
	}
}
