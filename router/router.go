package router

import (
	"net/http"
	"net/http/httputil"
	"strings"
)

type Router struct {
	router *httputil.ReverseProxy
}

func New() *Router {
	r := new(Router)

	// Reverse proxy that selects the backend by nearest match to the request URL
	// on the consistent hash ring.
	r.router = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			backend, alternates := r.Route(req)
			req.URL.Host = backend
			req.Header.Set("X-Backends", strings.Join(alternates, ","))
		},
	}

	return r
}

// Returns the nearest match and a set of alternates for any HTTP request.
func (r *Router) Route(req *http.Request) (string, []string) {
	// Use preset X-Backends header
	backends := strings.Split(req.Header.Get("X-Backends"), ",")
	return backends[0], backends[1:]
}

func (r *Router) Serve(port *string) {
	http.ListenAndServe(":"+*port, r.router)
}
