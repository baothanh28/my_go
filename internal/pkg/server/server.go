package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"myapp/internal/pkg/config"
	"myapp/internal/pkg/logger"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

// Server wraps Echo server
type Server struct {
	echo   *echo.Echo
	config *config.Config
	logger *logger.Logger
}

// NewEchoServer creates a new Echo server instance
func NewEchoServer(cfg *config.Config, log *logger.Logger) *Server {
	e := echo.New()

	// Hide Echo banner
	e.HideBanner = true
	e.HidePort = true

	// Configure Echo
	e.Server.ReadTimeout = time.Duration(cfg.Server.ReadTimeout) * time.Second
	e.Server.WriteTimeout = time.Duration(cfg.Server.WriteTimeout) * time.Second

	// Add middleware
	setupMiddleware(e, log)

	// Health check endpoint
	e.GET("/health", healthCheckHandler)

	log.Info("Echo server initialized")

	return &Server{
		echo:   e,
		config: cfg,
		logger: log,
	}
}

// setupMiddleware configures Echo middleware
func setupMiddleware(e *echo.Echo, log *logger.Logger) {
	// Recover middleware
	e.Use(middleware.Recover())

	// CORS middleware
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	// Request ID middleware
	e.Use(middleware.RequestID())

	// Custom logger middleware
	e.Use(requestLoggerMiddleware(log))

	// Timeout middleware
	e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		Timeout: 30 * time.Second,
	}))
}

// requestLoggerMiddleware creates a custom logger middleware
func requestLoggerMiddleware(log *logger.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			req := c.Request()
			res := c.Response()

			// Process request
			err := next(c)

			// Log request
			log.Info("HTTP request",
				zap.String("request_id", c.Response().Header().Get(echo.HeaderXRequestID)),
				zap.String("method", req.Method),
				zap.String("uri", req.RequestURI),
				zap.String("remote_ip", c.RealIP()),
				zap.Int("status", res.Status),
				zap.Int64("latency_ms", time.Since(start).Milliseconds()),
				zap.String("user_agent", req.UserAgent()),
			)

			return err
		}
	}
}

// healthCheckHandler handles health check requests
func healthCheckHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Server is healthy",
		"data": map[string]string{
			"status": "ok",
		},
	})
}

// GetEcho returns the Echo instance
func (s *Server) GetEcho() *echo.Echo {
	return s.echo
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	s.logger.Info("Starting HTTP server", zap.String("address", addr))
	return s.echo.Start(addr)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down HTTP server")
	return s.echo.Shutdown(ctx)
}

// Response is a standard API response structure
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   interface{} `json:"error,omitempty"`
	Message string      `json:"message"`
}

// SuccessResponse creates a success response
func SuccessResponse(c echo.Context, statusCode int, data interface{}, message string) error {
	return c.JSON(statusCode, Response{
		Success: true,
		Data:    data,
		Message: message,
	})
}

// ErrorResponse creates an error response
func ErrorResponse(c echo.Context, statusCode int, err interface{}, message string) error {
	return c.JSON(statusCode, Response{
		Success: false,
		Error:   err,
		Message: message,
	})
}
