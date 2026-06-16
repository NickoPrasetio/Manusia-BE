package main

import (
	_ "embed"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/manusia/api-gateway/internal/config"
)

//go:embed docs/openapi.yaml
var openapiSpec []byte

func main() {
	cfg := config.Load()

	authURL, _    := url.Parse(cfg.AuthServiceURL)
	userURL, _    := url.Parse(cfg.UserServiceURL)
	bookingURL, _ := url.Parse(cfg.BookingServiceURL)
	reviewURL, _  := url.Parse(cfg.ReviewServiceURL)
	chatURL, _    := url.Parse(cfg.ChatServiceURL)

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

	// ── Swagger UI ──────────────────────────────────────────────────────────────
	// Akses di http://localhost:8080/docs
	r.GET("/docs/spec", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", openapiSpec)
	})
	r.GET("/docs", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerUI))
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
		{"/api/chats", chatURL},
		{"/ws/chat", chatURL},
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
	log.Printf("Swagger UI  → http://localhost:%s/docs", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

const swaggerUI = `<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Manusia API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body { margin: 0; }
    .swagger-ui .topbar { background: #1e40af; }
    .swagger-ui .topbar .download-url-wrapper { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
  <script>
    window.onload = () => {
      SwaggerUIBundle({
        url: '/docs/spec',
        dom_id: '#swagger-ui',
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
        layout: 'StandaloneLayout',
        deepLinking: true,
        defaultModelsExpandDepth: 1,
        defaultModelExpandDepth: 1,
        displayRequestDuration: true,
        filter: true,
        tryItOutEnabled: true,
      });
    };
  </script>
</body>
</html>`
