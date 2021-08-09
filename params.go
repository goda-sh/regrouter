package regrouter

import "fmt"

// Params hold the HTTP params
type Params struct {
	Values map[string]string
}

// GetE returns a param or error
func (p Params) GetE(key string) (string, error) {
	if res, ok := p.Values[key]; ok {
		return res, nil
	}

	return "", fmt.Errorf("%q: no such param", key)
}

// Get returns a param or empty string
func (p Params) Get(key string) string {
	return p.Values[key]
}

// Set adds a param
func (p Params) Set(key string, value string) bool {
	p.Values[key] = value
	_, ok := p.Values[key]
	return ok
}
