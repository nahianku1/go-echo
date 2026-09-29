package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"net-http/internal/config"
	"net-http/internal/database"
	"net-http/internal/middlewares"
	"net-http/internal/modules/auth"
	"net-http/internal/modules/logs"
	"net-http/internal/utils"
)

func main() {
	// ------------------------------------------------------------------
	// Composition root.
	//
	// This is the only place that knows about concrete implementations. Every
	// service, controller and middleware below receives its collaborators as
	// constructor arguments and never reaches for a package-level global, so the
	// dependency graph is visible in one screen and any piece can be swapped
	// (a fake mailer in tests, a different database, another token signer)
	// without editing the code that uses it.
	// ------------------------------------------------------------------

	// Infrastructure adapters.
	db := database.ConnectDB()

	database.AutoMigrate(db, &auth.User{}, &auth.AuthAccount{}, &middlewares.RequestLog{})

	// One token signer is built from config and shared by everything that needs
	// to issue or verify tokens, so the secret is configured in exactly one place.
	tokens := utils.NewJWTSigner(config.AppConfig.JWT_SECRET)

	mailer, mailerErr := utils.NewSMTPMailer(config.AppConfig)

	if mailerErr != nil {
		utils.NewLogger("MAILER").Fatalf("Failed to create SMTP mailer: %v", mailerErr)
	}

	authGuard := middlewares.NewAuth(tokens)

	// Auth module: database + transport + signer in, HTTP controller out.
	authService := auth.NewService(db, mailer, tokens)
	authController := auth.NewController(
		authService,
		tokens,
		auth.NewGoogleOAuthConfig(
			config.AppConfig.GOOGLE_CLIENT_ID,
			config.AppConfig.GOOGLE_CLIENT_SECRET,
			config.AppConfig.GOOGLE_REDIRECT_URL,
		),
		config.AppConfig.GOOGLE_CLIENT_ID,
		config.AppConfig.FRONTEND_URL,
	)

	// Logs module.
	logsService := logs.NewService(db)
	logsController := logs.NewController(logsService)

	e := echo.New()
	// v5: Echo logs through log/slog; route it through the project's prefix style
	// by keeping the default slog handler and silencing banner output in StartConfig.
	e.Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Errors and panics surface through the project's standard JSON envelope.
	e.HTTPErrorHandler = middlewares.HTTPErrorHandler
	e.Use(middleware.Recover())

	// Request logging needs the database (to persist rows) and the token service
	// (to attribute a request to a user); both are injected here.
	e.Use(middlewares.NewRequestLogger(db, tokens).Middleware())

	// CORS middleware (echo built-in, same policy as the previous rs/cors setup).
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"http://localhost:5173"}, // Assuming default Vite port
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-Requested-With"},
		AllowCredentials: true,
	}))

	// Serve OpenAPI specification and interactive documentation UI
	e.GET("/api/openapi", func(c *echo.Context) error {
		return c.File("openapi.yaml")
	})

	e.GET("/api/docs", func(c *echo.Context) error {
		return c.File("docs.html")
	})

	// Modules register their own routes on the shared echo instance.
	authController.RegisterRoutes(e, authGuard)
	logsController.RegisterRoutes(e, authGuard)

	// ------------------------------------------------------------------
	// Server lifecycle, v5 style:
	//   - signal.NotifyContext turns SIGINT/SIGTERM into context cancellation
	//   - echo.StartConfig owns the http.Server, banner/port output and
	//     graceful shutdown (GracefulTimeout), replacing the old hand-rolled
	//     goroutine + server.Shutdown plumbing.
	// ------------------------------------------------------------------
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverAddr := net.JoinHostPort(config.AppConfig.HOST, config.AppConfig.PORT)
	utils.NewLogger("SERVER").Printf("Server started on http://%s:%s", config.AppConfig.HOST, config.AppConfig.PORT)

	sc := echo.StartConfig{
		Address:         serverAddr,
		HideBanner:      true,
		HidePort:        true, // our own log line above already announces the address
		GracefulTimeout: 10 * time.Second,
		OnShutdownError: func(err error) {
			utils.NewLogger("SERVER").Println("Graceful shutdown failed:", err)
		},
	}

	if err := sc.Start(ctx, e); err != nil {
		utils.NewLogger("SERVER").Fatalf("Server error: %v", err)
	}

	utils.NewLogger("SERVER").Printf("Server gracefully shutdown!")
}
