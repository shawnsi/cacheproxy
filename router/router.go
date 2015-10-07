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
			req.URL.Host = r.Route(req)
		},
	}

	return r
}

// Returns the nearest match and a set of alternates for any HTTP request.
func (r *Router) Route(req *http.Request) string {
	// Use preset X-Backends header
	split := strings.SplitN(req.Header.Get("X-Backends"), ",", 2)
	backend, alternates := split[0], split[1:]

	if len(backend) > 0 {
		req.Header.Set("X-Backends", strings.Join(alternates, ","))
		return backend
	} else {
		// This is purely for experimentation.  The actually routing should probably be built on vulcand or similar.
		return req.Header.Get("X-Origin")
	}
}

func (r *Router) Serve(port *string) {
	http.ListenAndServe(":"+*port, r.router)
}
