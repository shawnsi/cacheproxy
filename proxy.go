package main

import (
	"io"
	"net/http"
	"net/http/httputil"
	"path"
	"stathat.com/c/consistent"
	"strings"
)

// Builds the X-Backends header with a chain of nearest 3 backends on the
// consistent hash ring.
func BackendHeader(c *consistent.Consistent, name string) string {
	backends, _ := c.GetN(name, 3)
	// Need to shuffle these for reliable distribution of cache objects
	return strings.Join(backends, ",")
}

// Reverse proxy that selects the backend by nearest match to the request URL
// on the consistent hash ring.
func CacheProxy(c *consistent.Consistent) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			backends, _ := c.GetN(req.URL.Path, 3)
			req.URL.Host = backends[0]
			req.Header.Set("X-Backends", strings.Join(backends, ","))
		},
		Transport: &http.Transport{},
	}
}

func CacheProxyManager(c *consistent.Consistent) *http.ServeMux {
	manager := http.NewServeMux()

	// RESTful interface for cache members
	// GET /members/
	// 	  Returns a list of current cache members

	// PUT /members/member[:port]
	//    Adds "member" as a cache member

	// DELETE /members/member[:port]
	//    Delets "member" from cache ring

	manager.HandleFunc("/members/", func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == "PUT":
			c.Add(path.Base(req.URL.Path))
		case req.Method == "DELETE":
			c.Remove(path.Base(req.URL.Path))
		}
		io.WriteString(w, strings.Join(c.Members(), ","))
	})

	return manager
}

func main() {
	ring := consistent.New()

	// Replace with runtime arguments for initialization
	ring.Add("localhost:9091")
	ring.Add("localhost:9092")
	ring.Add("localhost:9093")
	ring.Add("localhost:9094")
	ring.Add("localhost:9095")

	// Initialize and run manager server in background via goroutine
	manager := CacheProxyManager(ring)
	go http.ListenAndServe(":9190", manager)

	proxy := CacheProxy(ring)
	http.ListenAndServe(":9090", proxy)
}
