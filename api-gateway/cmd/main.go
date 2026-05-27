package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/manusia/api-gateway/internal/config"
)

func main() {
	cfg := config.Load()

	authURL, _    := url.Parse(cfg.AuthServiceURL)
	userURL, _    := url.Parse(cfg.UserServiceURL)
	bookingURL, _ := url.Parse(cfg.BookingServiceURL)
	reviewURL, _  := url.Parse(cfg.ReviewServiceURL)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "api-gateway"})
	})

	// Route rules: path prefix → target URL
	routes := []struct {
		prefix string
		target *url.URL
	}{
		{"/api/auth", authURL},
		{"/api/users", userURL},
		{"/api/workers", userURL},
		{"/api/bookings", bookingURL},
		{"/api/jobs", bookingURL},
		{"/api/reviews", reviewURL},
		{"/api/internal", userURL},
	}

	// CORS headers that upstream services may send — strip them so the
	// gateway's own CORS middleware is the single source of truth.
	corsHeaders := []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers",
		"Access-Control-Expose-Headers",
		"Access-Control-Allow-Credentials",
		"Access-Control-Max-Age",
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		for _, route := range routes {
			if strings.HasPrefix(path, route.prefix) {
				proxy := httputil.NewSingleHostReverseProxy(route.target)

				// Strip upstream CORS headers to avoid duplicates
				proxy.ModifyResponse = func(resp *http.Response) error {
					for _, h := range corsHeaders {
						resp.Header.Del(h)
					}
					return nil
				}

				proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
					log.Printf("proxy error [%s]: %v", path, err)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadGateway)
					_, _ = w.Write([]byte(`{"error":"service tidak tersedia"}`))
				}
				// Fix the host header
				c.Request.Host = route.target.Host
				proxy.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		c.JSON(http.StatusNotFound, gin.H{"error": "route tidak ditemukan"})
	})

	log.Printf("api-gateway listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
