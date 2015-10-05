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

type CacheProxyManager struct {
	consistent *consistent.Consistent
	memberChan chan chan []string
}

func New() *CacheProxyManager {
	c := new(CacheProxyManager)
	c.consistent = consistent.New()

	return c
}

func (c *CacheProxyManager) Add(name string) {
	c.consistent.Add(name)
}

func (c *CacheProxyManager) Manager() *http.ServeMux {
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
			c.consistent.Add(path.Base(req.URL.Path))
		case req.Method == "DELETE":
			c.consistent.Remove(path.Base(req.URL.Path))
		}
		io.WriteString(w, strings.Join(c.Members(), ","))
	})

	return manager
}

func (c *CacheProxyManager) Members() []string {
	resultChan := make(chan []string)
	go func() {
		// This is not truly concurrent yet.
		resultChan <- c.consistent.Members()
	}()
	return <-resultChan
}

// Reverse proxy that selects the backend by nearest match to the request URL
// on the consistent hash ring.
func (c *CacheProxyManager) Proxy() *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			backends, _ := c.consistent.GetN(req.URL.Path, 3)
			shuffle(backends)
			req.URL.Host = backends[0]
			req.Header.Set("X-Backends", strings.Join(backends, ","))
		},
		Transport: &http.Transport{},
	}
}

func (c *CacheProxyManager) Remove(name string) {
	c.consistent.Remove(name)
}

func main() {
	manager := New()

	// Replace with runtime arguments for initialization
	manager.Add("localhost:9091")
	manager.Add("localhost:9092")
	manager.Add("localhost:9093")
	manager.Add("localhost:9094")
	manager.Add("localhost:9095")

	// Initialize and run manager server in background via goroutine
	go http.ListenAndServe(":9190", manager.Manager())

	http.ListenAndServe(":9090", manager.Proxy())
}
