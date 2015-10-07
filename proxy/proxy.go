package proxy

import (
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"path"
	"strings"

	"github.com/rcrowley/go-metrics"
	"stathat.com/c/consistent"
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
	replicas *int
	manager  *http.ServeMux
	proxy    *httputil.ReverseProxy
	registry metrics.Registry
	hits     metrics.Meter
	misses   metrics.Meter
	requests metrics.Meter
	timer    metrics.Timer
}

func New(replicas *int) *CacheProxy {
	c := new(CacheProxy)
	c.backends = consistent.New()
	c.replicas = replicas

	// Metric registry
	c.registry = metrics.NewRegistry()

	// Meters
	c.hits = metrics.NewMeter()
	c.misses = metrics.NewMeter()
	c.requests = metrics.NewMeter()
	c.registry.Register("hits", c.hits)
	c.registry.Register("misses", c.misses)
	c.registry.Register("requests", c.requests)

	// Backend response timer
	c.timer = metrics.NewTimer()
	c.registry.Register("backend", c.timer)

	// RESTful interface for cache members
	c.manager = http.NewServeMux()
	c.manager.HandleFunc("/members/", func(w http.ResponseWriter, req *http.Request) {
		backend := path.Base(req.URL.Path)
		switch {
		case req.Method == "PUT":
			// Adds "member" as a cache member
			log.Println("Adding backend: " + backend)
			c.backends.Add(backend)
		case req.Method == "DELETE":
			// Deletes "member" from cache ring
			log.Println("Removing backend: " + backend)
			c.backends.Remove(backend)
		}
		// Always returns a list of current cache members
		io.WriteString(w, strings.Join(c.backends.Members(), ","))
	})

	// RESTful interface for metrics
	c.manager.HandleFunc("/metrics/", func(w http.ResponseWriter, req *http.Request) {
		metrics.WriteOnce(c.registry, w)
	})

	// Reverse proxy that selects the backend by nearest match to the request URL
	// on the consistent hash ring.
	c.proxy = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			backend, alternates := c.Route(req)
			req.URL.Host = backend
			req.Header.Set("X-Backends", strings.Join(alternates, ","))
		},
		Transport: &MeteredTransport{cacheproxy: c},
	}

	return c
}

func (c *CacheProxy) Add(name string) {
	c.backends.Add(name)
}

// Returns the nearest match and a set of alternates for any HTTP request.
func (c *CacheProxy) Route(req *http.Request) (string, []string) {
	backends, _ := c.backends.GetN(req.URL.Path, *c.replicas)
	shuffle(backends)
	return backends[0], backends[1:]
}

func (c *CacheProxy) Manage(port *string) {
	http.ListenAndServe(":"+*port, c.manager)
}

func (c *CacheProxy) Serve(port *string) {
	http.ListenAndServe(":"+*port, c.proxy)
}
