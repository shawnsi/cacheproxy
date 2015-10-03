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

// Reverse proxy that selects the backend by nearest match to the request URL
// on the consistent hash ring.
func CacheProxy(c *consistent.Consistent) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			backends, _ := c.GetN(req.URL.Path, 3)
			shuffle(backends)
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
