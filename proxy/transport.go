package proxy

import (
	"net/http"
)

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
