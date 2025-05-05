package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	csrf "github.com/utrack/gin-csrf"
	"golang.org/x/text/language"

	"github.com/schoolboylurk/data-sentinel/pkg/ai"
	"github.com/schoolboylurk/data-sentinel/pkg/auth"
	"github.com/schoolboylurk/data-sentinel/pkg/database"
	"github.com/schoolboylurk/data-sentinel/pkg/handlers"
	"github.com/schoolboylurk/data-sentinel/pkg/middleware"
)

var bundle *i18n.Bundle

func initI18n() {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.LoadMessageFile("web/locales/en.json")
	bundle.LoadMessageFile("web/locales/es.json")
}

// I18nMiddleware sets up a localizer based on Accept-Language
func I18nMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := c.GetHeader("Accept-Language")
		localizer := i18n.NewLocalizer(bundle, lang)
		c.Set("localizer", localizer)
		c.Next()
	}
}

func main() {
	// 1. ENV checks
	required := []string{"OPENAI_API_KEY", "PERMIT_API_KEY", "PERMIT_PDP_URL", "SESSION_SECRET", "DB_PATH", "DB_SCHEMA"}
	for _, e := range required {
		if os.Getenv(e) == "" {
			log.Fatalf("%s must be set", e)
		}
	}

	// 2. Init SDKs and database
	auth.InitPermit()
	ai.InitOpenAI()
	dbPath := os.Getenv("DB_PATH")
	schema := os.Getenv("DB_SCHEMA")
	if dbPath == "" || schema == "" {
		log.Fatal("DB_PATH and DB_SCHEMA must be set")
	}
	if err := database.InitDB(dbPath, schema); err != nil {
		log.Fatalf("DB init failed: %v", err)
	}
	initI18n()

	// 3. Gin setup
	r := gin.Default()

	// 3a. Sessions
	store := cookie.NewStore([]byte(os.Getenv("SESSION_SECRET")))
	// configure secure cookie options
	store.Options(sessions.Options{
		Path:     "/",                        // valid for entire site
		Domain:   os.Getenv("COOKIE_DOMAIN"), // e.g. ".example.com" or leave empty for host-based
		MaxAge:   60 * 60 * 24,               // 1 day
		Secure:   true,                       // true = only sent over HTTPS
		HttpOnly: true,                       // not accessible from JS
		SameSite: http.SameSiteLaxMode,       // or StrictMode
	})
	r.Use(sessions.Sessions("ai-session", store))

	// 3b. CSRF protection
	r.Use(csrf.Middleware(csrf.Options{
		Secret: os.Getenv("SESSION_SECRET"),
		ErrorFunc: func(c *gin.Context) {
			c.String(http.StatusForbidden, "CSRF token mismatch")
			c.Abort()
		},
	}))

	// 3c. Internationalization
	r.Use(I18nMiddleware())
	r.SetFuncMap(template.FuncMap{
		"T": func(c *gin.Context, key string) string {
			loc := c.MustGet("localizer").(*i18n.Localizer)
			msg, _ := loc.Localize(&i18n.LocalizeConfig{MessageID: key})
			return msg
		},
	})

	// 3d. Templates & Static files
	r.LoadHTMLGlob("web/templates/*.html")
	r.Static("/static", "./web/static")

	// health endpoint for Docker HEALTHCHECK
	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 4. Admin login & UI routes
	r.GET("/login", handlers.ShowLogin)
	r.POST("/login", handlers.PerformLogin)

	admin := r.Group("/admin")
	admin.Use(handlers.AdminRequired)
	admin.GET("/kids", handlers.ListKidsPage)
	admin.POST("/kids", handlers.AddKid)
	admin.GET("/policies", handlers.ListPoliciesPage)
	admin.POST("/policies", handlers.UpdatePolicy)
	admin.GET("/requests", handlers.ListRequestsPage)
	admin.POST("/approve/:id", handlers.ApprovePromptHandler)
	admin.GET("/dashboard", handlers.ShowAdminDashboard)
	admin.GET("/metrics", handlers.MetricsHandler)
	admin.GET("/violations", handlers.ViolationMetrics)
	admin.GET("/groups", handlers.ListGroupsPage)
	admin.POST("/groups", handlers.AddGroup)
	admin.POST("/groups/:id/members", handlers.AddMember)

	// 5. Child UI & chat endpoints
	r.GET("/child/login", handlers.ShowChildLogin)
	r.POST("/child/login", handlers.PerformChildLogin)

	child := r.Group("/child", handlers.ChildRequired)
	child.GET("/chat", handlers.ShowChildChatPage)                                   // persistent chat UI
	child.POST("/session", handlers.StartChatSession)                                // create session
	child.POST("/session/:id/message", middleware.RateLimit(), handlers.PostMessage) // post message
	child.GET("/session/:id/history", handlers.GetChatHistory)                       // fetch history

	// 6. API endpoints for programmatic use
	r.POST("/request-prompt", handlers.RequestPromptHandler)
	r.POST("/approve/:id", handlers.ApprovePromptHandler)
	r.POST("/generate-report", handlers.GenerateReportHandler)

	// 7. Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Listening on :%s", port)
	log.Fatal(r.Run(":" + port))
}
