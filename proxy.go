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
}

func New() *CacheProxy {
	c := new(CacheProxy)
	c.backends = consistent.New()
	return c
}

func (c *CacheProxy) Manager() *http.ServeMux {
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
			c.backends.Add(path.Base(req.URL.Path))
		case req.Method == "DELETE":
			c.backends.Remove(path.Base(req.URL.Path))
		}
		io.WriteString(w, strings.Join(c.backends.Members(), ","))
	})

	return manager
}

func (c *CacheProxy) Select(path string) []string {
	selection, _ := c.backends.GetN(path, 3)
	shuffle(selection)
	return selection
}

// Reverse proxy that selects the backend by nearest match to the request URL
// on the consistent hash ring.
func (c *CacheProxy) Proxy() *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			backends := c.Select(req.URL.Path)
			req.URL.Host = backends[0]
			req.Header.Set("X-Backends", strings.Join(backends, ","))
		},
		Transport: &http.Transport{},
	}
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
	go http.ListenAndServe(":9190", proxy.Manager())

	http.ListenAndServe(":9090", proxy.Proxy())
}
