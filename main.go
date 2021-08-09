package regrouter

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// RegRouter is the RegRouter instance
type RegRouter struct {
	Routes   []Route
	CTX      struct{}
	Handlers Handlers
}

// Route is the http routes
type Route struct {
	method  string
	regex   *regexp.Regexp
	handler http.HandlerFunc
	CORS    bool
}

// Handlers are default error code handlers + CORS
type Handlers struct {
	CORS       func([]string, http.ResponseWriter, *http.Request)
	ErrorCodes map[int]func(map[string]interface{}, http.ResponseWriter, *http.Request)
}

// New returns a RegRouter instance
func New() *RegRouter {
	return &RegRouter{
		Handlers: Handlers{
			// Default CORS response
			CORS: func(methods []string, w http.ResponseWriter, r *http.Request) {
				headers := w.Header()
				headers.Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
				headers.Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))
				headers.Set("Access-Control-Allow-Credentials", "true")
				headers.Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			},
			// Default error handlers
			ErrorCodes: map[int]func(map[string]interface{}, http.ResponseWriter, *http.Request){
				404: func(data map[string]interface{}, w http.ResponseWriter, r *http.Request) {
					code := http.StatusNotFound
					http.Error(w, fmt.Sprintf("%d - %s\n", code, http.StatusText(code)), code)
				},
				405: func(data map[string]interface{}, w http.ResponseWriter, r *http.Request) {
					code := http.StatusMethodNotAllowed
					w.Header().Set("Allow", data["allowed"].(string))
					http.Error(w, fmt.Sprintf("%d - %s (Valid: %s)\n", code, http.StatusText(code), data["allowed"]), code)
				},
				500: func(data map[string]interface{}, w http.ResponseWriter, r *http.Request) {
					code := http.StatusInternalServerError
					http.Error(w, fmt.Sprintf("%d - %s (Exception: %s)\n", code, http.StatusText(code), data["exception"]), code)
				},
			},
		},
	}
}

// Handler returns an HTTP handler
func (rr *RegRouter) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Attempt to recovery from any errors for a 500 error response
		defer func() {
			if err := recover(); err != nil {
				rr.Handlers.ErrorCodes[500](map[string]interface{}{"exception": err}, w, r)
			}
		}()

		var (
			methods []string // Allowed CORS methods.
			allowed []string // Allowed request methods.
			params  = Params{map[string]string{}}
		)

		// Loop each route.
		for _, route := range rr.Routes {
			// Test each route for matching regex to the request URL path.
			matches := route.regex.FindStringSubmatch(r.URL.Path)
			if len(matches) > 0 {
				// Add the method to the allowed CORS request methods
				if route.CORS {
					methods = append(methods, route.method)
					if !strings.EqualFold(r.Method, route.method) {
						allowed = append(allowed, route.method)
						continue
					}

					// Send CORS headers
					rr.Handlers.CORS(methods, w, r)
				}

				// Build a list url params based on named regex AND/OR index
				for i, name := range route.regex.SubexpNames() {
					params.Set(map[bool]string{
						true:  fmt.Sprintf("%d", i),
						false: name,
					}[len(name) == 0], matches[i])
				}

				// Run the request handler
				route.handler(w, r.WithContext(context.WithValue(r.Context(), rr.CTX, params)))
				return
			}
		}

		// Handle CORS or provide a list of allowed request methods
		if len(methods) > 0 {
			rr.Handlers.CORS(methods, w, r)
			w.WriteHeader(http.StatusNoContent)
			return
		} else if len(allowed) > 0 {
			rr.Handlers.ErrorCodes[405](map[string]interface{}{"allowed": strings.Join(allowed, ", ")}, w, r)
			return
		}

		// Handle a 404 error message
		rr.Handlers.ErrorCodes[404](map[string]interface{}{}, w, r)
	})
}

// Add adds a route to the RegRouter
func (rr *RegRouter) Add(method string, pattern string, handler http.HandlerFunc, cors bool) {
	rr.Routes = append(rr.Routes, Route{strings.ToUpper(method), regexp.MustCompile("^" + pattern + "$"), handler, cors})
}

// Static servers static files
func (rr *RegRouter) Static(path string, pattern string) {
	rr.Add("GET", pattern, func(w http.ResponseWriter, r *http.Request) {
		if file, err := rr.Params(r).GetE("filepath"); err == nil {
			r.URL.Path = fmt.Sprintf("/%s", file)
			http.FileServer(http.Dir(path)).ServeHTTP(w, r)
		}
	}, false)
}

// Params is a helper to get request paramaters
func (rr *RegRouter) Params(r *http.Request) Params {
	return r.Context().Value(rr.CTX).(Params)
}
