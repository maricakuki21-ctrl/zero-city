package handler

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// Harness remains internal; callers enter through the existing JWT user routes.
func (h *BizDecipherHandler) ProxyHarness(c *gin.Context) {
	target := &url.URL{Scheme: "http", Host: "127.0.0.1:18082"}
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api/v1/biz/harness")
		r.URL.RawPath = ""
		r.Header.Del("Cookie")
	}
	proxy.FlushInterval = -1
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"Harness 工作台暂未连接"}`))
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}
