package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func BenchmarkParseJSONC(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		doc := benchmarkConfigJSON(size)
		b.Run(fmt.Sprintf("apis=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(doc)))
			for i := 0; i < b.N; i++ {
				if _, err := parseJSONC(doc); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkEncodingJSONUnmarshal(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		doc := benchmarkConfigJSON(size)
		b.Run(fmt.Sprintf("apis=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(doc)))
			for i := 0; i < b.N; i++ {
				var v any
				if err := json.Unmarshal(doc, &v); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkJSONCSetPath(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		doc := benchmarkConfigJSONC(size)
		path := []string{"apis", "api0000", "base_url"}
		b.Run(fmt.Sprintf("apis=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := jsoncSetPath(doc, path, "https://patched.example.com"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkJSONCDeletePath(b *testing.B) {
	for _, size := range []int{10, 100, 1000} {
		doc := benchmarkConfigJSONC(size)
		path := []string{"apis", fmt.Sprintf("api%04d", size-1)}
		b.Run(fmt.Sprintf("apis=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := jsoncDeletePath(doc, path); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchmarkConfigJSON(count int) []byte {
	var s strings.Builder
	s.Grow(count * 150)
	s.WriteString("{\n  \"apis\": {\n")
	for i := 0; i < count; i++ {
		if i > 0 {
			s.WriteString(",\n")
		}
		fmt.Fprintf(&s, "    \"api%04d\": {\n", i)
		fmt.Fprintf(&s, "      \"base_url\": \"https://api%04d.example.com\",\n", i)
		s.WriteString("      \"profiles\": {\n")
		s.WriteString("        \"default\": {\n")
		s.WriteString("          \"auth\": {\n")
		s.WriteString("            \"type\": \"bearer\",\n")
		s.WriteString("            \"params\": {\n")
		fmt.Fprintf(&s, "              \"token\": \"secret-%04d\"\n", i)
		s.WriteString("            }\n")
		s.WriteString("          }\n")
		s.WriteString("        }\n")
		s.WriteString("      }\n")
		s.WriteString("    }")
	}
	s.WriteString("\n  }\n}\n")
	return []byte(s.String())
}

func benchmarkConfigJSONC(count int) []byte {
	var s strings.Builder
	s.Grow(count * 170)
	s.WriteString("{\n  // registered APIs\n  \"apis\": {\n")
	for i := 0; i < count; i++ {
		if i > 0 {
			s.WriteString(",\n")
		}
		fmt.Fprintf(&s, "    // api %04d\n", i)
		fmt.Fprintf(&s, "    \"api%04d\": {\n", i)
		fmt.Fprintf(&s, "      \"base_url\": \"https://api%04d.example.com\" // keep\n", i)
		s.WriteString("    }")
	}
	s.WriteString("\n  }\n}\n")
	return []byte(s.String())
}
