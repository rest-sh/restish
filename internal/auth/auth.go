package auth

import "net/http"

// Param describes a configuration parameter required by an auth handler.
type Param struct {
	Name        string
	Description string
	Required    bool
	Secret      bool // true → don't echo when prompting
}

// Handler is implemented by each auth mechanism.
type Handler interface {
	// Parameters returns the list of configuration parameters this handler needs.
	Parameters() []Param
	// OnRequest mutates req to add authentication credentials.
	// params contains values from the profile's auth config.
	OnRequest(req *http.Request, params map[string]string) error
}
