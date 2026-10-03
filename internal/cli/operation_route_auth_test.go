package cli

import (
	"testing"

	"github.com/rest-sh/restish/v2/internal/spec"
)

func TestConditionalSecurityPathParameterMatchesSlashContainingGenericRequest(t *testing.T) {
	rules := []spec.ConditionalSecurityRule{{
		When:         spec.ConditionalSecurityPredicate{PathParameter: "path", Prefix: ".github/workflows/"},
		Alternatives: []int{1},
	}}
	template := "/github/repos/{owner}/{repo}/contents/{path}"
	requestPath := "/github/repos/acme/widgets/contents/.github/workflows/release.yml"

	if _, ok := routeTemplateMatchScore(template, requestPath, conditionalSecurityPathParameters(rules)); !ok {
		t.Fatal("conditional path parameter did not consume the remaining generic request path")
	}
	if _, ok := routeTemplateMatchScore(template, requestPath, nil); ok {
		t.Fatal("ordinary OpenAPI path parameters must still consume exactly one segment")
	}
}
