package server

import (
	"context"
	"log"
	"net/http"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/config"
	sqlc_auth "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/auth"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/handler"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/middleware"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/service"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	ws "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/websocket"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var manager = ws.NewClientManager()

type WebSocketServer struct {
	route  *gin.Engine
	config *config.Config
}

func NewWebSocketServer(ctx context.Context, db sqlc_auth.Querier, rdb *redis.Client) *WebSocketServer {
	apiGatewayCertFile := utils.GetEnv("PATH_CERT_API_GATEWAY_CLIENT", "")
	apiGatewayKeyFile := utils.GetEnv("PATH_KEY_API_GATEWAY_CLIENT", "")

	r := gin.Default()

	websocketConfig := config.NewConfigWebsocket()

	userClient, err := client.NewUserClient(websocketConfig.Service.UserServiceAddr, apiGatewayCertFile, apiGatewayKeyFile)
	if err != nil {
		panic("Failed to create UserClient: " + err.Error())
	}

	userService := service.NewWebsocketService(userClient)

	websocketHandler := handler.NewWebSocketHandler(userService)

	r.Use(middleware.CORSMiddleware(),
		middleware.RateLimitMiddleware(ctx, rdb, 60, 100), // 100 requests per 60 seconds
		middleware.LoggerMiddleware(),
		middleware.AuthMiddleware(db, rdb),
	)

	r.GET("/ws", websocketHandler.HandleWebsocket)

	return &WebSocketServer{
		route:  r,
		config: websocketConfig,
	}
}

func (s *WebSocketServer) RunTLS(ctx context.Context) (string, error) {
	// 1. Start server with shut down gracefully
	srv := &http.Server{
		Addr:    ":" + s.config.WebsocketServer.Port,
		Handler: h2c.NewHandler(s.route, &http2.Server{}),

		ReadTimeout:       s.config.WebsocketServer.ReadTimeout,
		ReadHeaderTimeout: s.config.WebsocketServer.ReadHeaderTimeout,
		WriteTimeout:      s.config.WebsocketServer.WriteTimeout,
		IdleTimeout:       s.config.WebsocketServer.IdleTimeout,

		MaxHeaderBytes: s.config.WebsocketServer.MaxHeaderBytes,
	}

	// 2. Create a channel to listen for server errors
	errChan := make(chan error, 1)

	// 3. Listen and serve in a goroutine
	go func() {
		log.Printf("Websocket is running on port %s...", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// 4. Wait for an error or a shutdown signal
	select {
	case err := <-errChan:
		return "Websocket error", err

	case <-ctx.Done():
		log.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.WebsocketServer.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			manager.CloseAll()
			return "Websocket forced to shutdown", err
		}

		manager.CloseAll()
		return "Websocket exiting gracefully!", nil
	}
}
