package main

import (
	"flag"
	"io"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"path"
	"strings"

	"github.com/rcrowley/go-metrics"
	"stathat.com/c/consistent"
)

var managerPort = flag.String("m", "9190", "manager port")
var proxyPort = flag.String("p", "9090", "proxy port")
var replicas = flag.Int("r", 3, "cache replica count")

// Sattolo shuffle
// https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle#Sattolo.27s_algorithm
func shuffle(list []string) {
	for i := range list {
		j := rand.Intn(i + 1)
		list[i], list[j] = list[j], list[i]
	}
}

// Embeds http.Transport for use along with metrics registry
type MeteredTransport struct {
	http.Transport
	cacheproxy *CacheProxy
}

// Shadows http.Transport.RoundTrip method with meter and timer updates
func (t *MeteredTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	t.cacheproxy.requests.Mark(1)

	// Time the real transport method
	t.cacheproxy.timer.Time(func() {
		resp, err = t.Transport.RoundTrip(req)
	})

	// Mark the appropriate cache response meter
	switch {
	case resp.Header.Get("X-Cache") == "HIT":
		t.cacheproxy.hits.Mark(1)
	default:
		t.cacheproxy.misses.Mark(1)
	}

	return
}

type CacheProxy struct {
	backends *consistent.Consistent
	manager  *http.ServeMux
	proxy    *httputil.ReverseProxy
	registry metrics.Registry
	hits     metrics.Meter
	misses   metrics.Meter
	requests metrics.Meter
	timer    metrics.Timer
}

func New() *CacheProxy {
	c := new(CacheProxy)
	c.backends = consistent.New()

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

// Returns the nearest match and a set of alternates for any HTTP request.
func (c *CacheProxy) Route(req *http.Request) (string, []string) {
	var backends []string

	switch {
	case req.Header.Get("X-Backends") != "":
		// This needs to sanitize input from potentially untrusted sources
		// Use preset X-Backends header
		backends = strings.Split(req.Header.Get("X-Backends"), ",")
	default:
		// Generate new array of backends
		backends, _ = c.backends.GetN(req.URL.Path, *replicas)
		shuffle(backends)
	}

	return backends[0], backends[1:]
}

func main() {
	flag.Parse()
	proxy := New()

	// Pass all remaining arguments in as backend
	backends := flag.Args()
	for index := range backends {
		proxy.backends.Add(backends[index])
	}

	// Initialize and run manager server in background via goroutine
	go http.ListenAndServe(":"+*managerPort, proxy.manager)

	http.ListenAndServe(":9090", proxy.proxy)
}
