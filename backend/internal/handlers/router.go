package handlers

import (
	"log/slog"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"nowdone/internal/config"
	"nowdone/internal/repository"
	"nowdone/internal/service"
)

// Deps bundles everything the router needs to wire up routes.
type Deps struct {
	Cfg       *config.Config
	Log       *slog.Logger
	Auth      *service.AuthService
	Users     *repository.UserRepository
	Tasks     *service.TaskService
	TaskTypes *service.TaskTypeService
	Notes     *service.NoteService
	S3        *service.S3Service
	AuthH     *AuthHandler
}

// NewRouter builds the Gin engine with all API routes registered.
func NewRouter(d Deps) *gin.Engine {
	if d.Cfg.Env != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger(d.Log))

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{d.Cfg.SiteURL},
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		AllowCredentials: true,
	}))

	authHandler := d.AuthH
	taskHandler := NewTaskHandler(d.Tasks, d.Users, d.Log)
	taskTypeHandler := NewTaskTypeHandler(d.TaskTypes, d.Log)
	noteHandler := NewNoteHandler(d.Notes, d.Log)
	uploadHandler := NewUploadHandler(d.S3, d.Log)

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	auth := r.Group("/auth")
	{
		auth.POST("/login/start", authHandler.StartLogin)
		auth.POST("/login/redeem", authHandler.RedeemLogin)
		auth.POST("/logout", authHandler.Logout)
	}

	protected := r.Group("/")
	protected.Use(AuthMiddleware(d.Auth, d.Log))
	{
		protected.GET("/auth/me", authHandler.Me)
		protected.PATCH("/auth/me", authHandler.UpdateSettings)
		protected.POST("/auth/pin", authHandler.SetPIN)
		protected.POST("/auth/pin/verify", authHandler.VerifyPIN)
		protected.POST("/auth/pin/reset/start", authHandler.StartPINReset)
		protected.POST("/auth/pin/reset/redeem", authHandler.RedeemPINReset)
		protected.POST("/auth/pin/reset/confirm", authHandler.SetNewPIN)

		protected.GET("/tasks", taskHandler.List)
		protected.POST("/tasks", taskHandler.Create)
		protected.PATCH("/tasks/:id", taskHandler.Update)
		protected.DELETE("/tasks/:id", taskHandler.Delete)

		protected.GET("/task-types", taskTypeHandler.List)
		protected.POST("/task-types", taskTypeHandler.Create)
		protected.DELETE("/task-types/:id", taskTypeHandler.Delete)

		protected.GET("/notes", noteHandler.List)
		protected.POST("/notes", noteHandler.Create)
		protected.PATCH("/notes/:id", noteHandler.Update)
		protected.DELETE("/notes/:id", noteHandler.Delete)

		protected.POST("/uploads/presign", uploadHandler.Presign)  // direct browser → S3
		protected.POST("/uploads", uploadHandler.Upload)           // legacy fallback: browser → API → S3
		protected.DELETE("/uploads", uploadHandler.DeleteOrphans)  // purge never-saved uploads
	}

	return r
}

func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		log.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
		)
	}
}
