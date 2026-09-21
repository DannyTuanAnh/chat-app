package server

import (
	"context"
	"encoding/json"
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
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// var manager = ws.NewClientManager()

type WebSocketServer struct {
	route   *gin.Engine
	config  *config.Config
	manager *ws.ClientManager

	rdb *redis.Client
}

func NewWebSocketServer(ctx context.Context, db sqlc_auth.Querier, rdb *redis.Client) *WebSocketServer {
	websocketCertFile := utils.GetEnv("PATH_CERT_WEBSOCKET_CLIENT", "")
	websocketKeyFile := utils.GetEnv("PATH_KEY_WEBSOCKET_CLIENT", "")

	r := gin.Default()

	websocketConfig := config.NewConfigWebsocket()

	userClient, err := client.NewUserClient(websocketConfig.Service.UserServiceAddr, websocketCertFile, websocketKeyFile)
	if err != nil {
		panic("Failed to create UserClient: " + err.Error())
	}

	manager := ws.NewClientManager()

	userService := service.NewWebsocketService(userClient)

	websocketHandler := handler.NewWebSocketHandler(userService, manager)

	r.Use(
		middleware.CORSMiddleware(),
		middleware.RateLimitMiddleware(ctx, rdb, 60, 100), // 100 requests per 60 seconds
		middleware.LoggerMiddleware(),
		middleware.AuthMiddleware(db, rdb),
	)

	r.GET("/ws", websocketHandler.HandleWebsocket)

	return &WebSocketServer{
		route:   r,
		config:  websocketConfig,
		manager: manager,
		rdb:     rdb,
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

	pathCert := utils.GetEnv("PATH_CERT_WEBSOCKET", "")
	pathKey := utils.GetEnv("PATH_KEY_WEBSOCKET", "")

	// 3. Listen and serve in a goroutine
	go func() {
		log.Printf("Websocket is running on port %s...", srv.Addr)
		if err := srv.ListenAndServeTLS(pathCert, pathKey); err != nil && err != http.ErrServerClosed {
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
			s.manager.CloseAll()
			return "Websocket forced to shutdown", err
		}

		s.manager.CloseAll()
		return "Websocket exiting gracefully!", nil
	}
}

func (s *WebSocketServer) StartRedisListener(ctx context.Context) {
	pubsub := s.rdb.Subscribe(ctx, "chat_notifications")
	defer pubsub.Close()

	if _, err := pubsub.Receive(ctx); err != nil {
		log.Printf("Failed to subscribe to Redis channel: %v", err)
		return
	}

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			log.Println("Redis listener context done. Redis listener shutting down...")
			return
		case msg, ok := <-ch:
			if !ok {
				log.Println("Redis subscription channel closed. Redis listener shutting down...")
				return
			}

			s.handleRedisMessage(ctx, msg.Payload)
		}
	}
}

func (s *WebSocketServer) handleRedisMessage(ctx context.Context, msg string) {
	log.Printf("Received message from Redis channel: %s", msg)

	var event ws.RealtimeEvent

	if err := json.Unmarshal([]byte(msg), &event); err != nil {
		log.Printf("Failed to unmarshal message: %v", err)
		return
	}

	switch event.Event {
	case "friend_request_accepted":
		outboundData, err := json.Marshal(event.Message)
		if err != nil {
			log.Printf("Failed to marshal outbound message: %v", err)
			return
		}

		argSendToUsersParams := ws.SendToUsersParams{
			UserIDs:     event.UserIDs,
			MessageType: websocket.MessageText,
			Data:        outboundData,
		}

		err = s.manager.SendToUsers(ctx, argSendToUsersParams)
		if err != nil {
			log.Printf("Failed to send message to users via WebSocket: %v", err)
		} else {
			log.Printf("Successfully sent message to users via WebSocket: %v", event.UserIDs)
		}
	}
}
