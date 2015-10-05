package main

import (
	"io"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"path"
	"stathat.com/c/consistent"
	"strings"
)

// Sattolo shuffle
// https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle#Sattolo.27s_algorithm
func shuffle(list []string) {
	for i := range list {
		j := rand.Intn(i + 1)
		list[i], list[j] = list[j], list[i]
	}
}

type CacheProxy struct {
	backends *consistent.Consistent
	manager  *http.ServeMux
	proxy    *httputil.ReverseProxy
}

func New() *CacheProxy {
	c := new(CacheProxy)
	c.backends = consistent.New()

	// RESTful interface for cache members
	c.manager = http.NewServeMux()
	c.manager.HandleFunc("/members/", func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == "PUT":
			// Adds "member" as a cache member
			c.backends.Add(path.Base(req.URL.Path))
		case req.Method == "DELETE":
			// Deletes "member" from cache ring
			c.backends.Remove(path.Base(req.URL.Path))
		}
		// Always returns a list of current cache members
		io.WriteString(w, strings.Join(c.backends.Members(), ","))
	})

	// Reverse proxy that selects the backend by nearest match to the request URL
	// on the consistent hash ring.
	c.proxy = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			backends := c.Select(req.URL.Path)
			req.URL.Host = backends[0]
			req.Header.Set("X-Backends", strings.Join(backends, ","))
		},
		Transport: &http.Transport{},
	}

	return c
}

func (c *CacheProxy) Select(path string) []string {
	selection, _ := c.backends.GetN(path, 3)
	shuffle(selection)
	return selection
}

func main() {
	proxy := New()

	// Replace with runtime arguments for initialization
	proxy.backends.Add("localhost:9091")
	proxy.backends.Add("localhost:9092")
	proxy.backends.Add("localhost:9093")
	proxy.backends.Add("localhost:9094")
	proxy.backends.Add("localhost:9095")

	// Initialize and run manager server in background via goroutine
	go http.ListenAndServe(":9190", proxy.manager)

	http.ListenAndServe(":9090", proxy.proxy)
}
