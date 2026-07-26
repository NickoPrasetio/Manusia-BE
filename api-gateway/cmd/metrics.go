package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"service", "method", "path", "status"})

	httpRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	}, []string{"service", "method", "path"})
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
}

func metricsMiddleware(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()

		// Gateway menggunakan NoRoute sehingga c.FullPath() selalu "".
		// Normalisasi ke dua segmen pertama (/api/bookings, /api/auth, dll)
		// untuk menghindari high cardinality dari path yang mengandung ID.
		path := c.FullPath()
		if path == "" {
			parts := strings.SplitN(strings.TrimPrefix(c.Request.URL.Path, "/"), "/", 3)
			if len(parts) >= 2 {
				path = "/" + parts[0] + "/" + parts[1]
			} else {
				path = c.Request.URL.Path
			}
		}

		httpRequestsTotal.WithLabelValues(
			serviceName,
			c.Request.Method,
			path,
			strconv.Itoa(c.Writer.Status()),
		).Inc()
		httpRequestDuration.WithLabelValues(
			serviceName,
			c.Request.Method,
			path,
		).Observe(time.Since(start).Seconds())
	}
}
