package webui

import (
	_ "embed"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

//go:embed index.html
var indexHTML string

// newReverseProxy builds a reverse proxy forwarding requests to
// the control server at the given listening address, injecting
// the API key header on every forwarded request.
func newReverseProxy(controlServerAddress, apiKey string) (proxy *httputil.ReverseProxy, err error) {
	host := "127.0.0.1"
	if !strings.HasPrefix(controlServerAddress, ":") {
		host, _, err = net.SplitHostPort(controlServerAddress)
		if err != nil {
			return nil, err
		}
	}

	port := strings.TrimPrefix(controlServerAddress, host)
	target, err := url.Parse("http://" + host + port)
	if err != nil {
		return nil, err
	}

	proxy = httputil.NewSingleHostReverseProxy(target)
	defaultDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		defaultDirector(req)
		if apiKey != "" {
			req.Header.Set("X-API-Key", apiKey)
		}
	}
	return proxy, nil
}

type handler struct {
	proxy *httputil.ReverseProxy
}

func newHandler(proxy *httputil.ReverseProxy) http.Handler {
	return &handler{proxy: proxy}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/v1/"):
		h.proxy.ServeHTTP(w, r)
	case r.URL.Path == "/" || r.URL.Path == "/index.html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
	case r.URL.Path == "/favicon.ico":
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}
