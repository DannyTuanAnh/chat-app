package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/config"
	sqlc_auth "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/auth"
	auth_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/auth"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/routes"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/sse"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
)

type ModelHTTP interface {
	Routes() routes.Routes
}

type Clients struct {
	AuthClient auth_proto.AuthServiceClient
}

type Application struct {
	config  *config.Config
	route   *gin.Engine
	modules []ModelHTTP

	Clients *Clients
}

// sqlc_auth for middleware, check api_key for public api, check session for private api
func NewApplication(ctx context.Context, db sqlc_auth.Querier, rdb *redis.Client) *Application {
	cfg := config.NewConfig()

	// 1. Initialize the Gin router
	r := gin.Default()

	// 2. Initialize custom validator
	err := validation.InitValidator()
	if err != nil {
		log.Fatalf("Failed to initialize validator: %v", err)
		return nil
	}

	// 3. Set up initial data in Redis, such as API key generation counter
	if err := rdb.SetNX(ctx, "generation_api_key", 1, 0).Err(); err != nil {
		log.Fatalf("Failed to set initial value in Redis: %v", err)
		return nil
	}

	// 4. Initialize modules
	modules := []ModelHTTP{
		NewAuthModule(cfg.Service.AuthServiceAddr),
		NewUserModule(cfg.Service.UserServiceAddr, cfg.Service.FriendServiceAddr, ctx, rdb),
		NewFriendModule(cfg.Service.FriendServiceAddr, ctx),
		NewNotifyModule(cfg.Service.UserServiceAddr),
	}

	// 5. Register all routes from modules by calling the getModuleRoutes helper function to extract the routes from each module
	// and then passing them to the routes.RegisterRoutes function to register them with the Gin router
	routes.RegisterRoutes(ctx, r, rdb, db, getModuleRoutes(modules)...)

	return &Application{
		config:  cfg,
		route:   r,
		modules: modules,
	}
}

func (ac *Application) Run(ctx context.Context) (string, error) {
	log.Println("ENV PORT:", os.Getenv("PORT"))
	log.Println("CONFIG PORT:", ac.config.Server.Port)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 1. Start server with shut down gracefully
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: h2c.NewHandler(ac.route, &http2.Server{}),

		ReadTimeout:       ac.config.Server.ReadTimeout,
		ReadHeaderTimeout: ac.config.Server.ReadHeaderTimeout,
		WriteTimeout:      ac.config.Server.WriteTimeout,
		IdleTimeout:       ac.config.Server.IdleTimeout,

		MaxHeaderBytes: ac.config.Server.MaxHeaderBytes,
	}
	log.Printf("Server is running on port %s...", srv.Addr)

	// 2. Create a channel to listen for server errors
	errChan := make(chan error, 1)

	// 3. Listen and serve in a goroutine
	go func() {
		log.Printf("Server is running on port %s...", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// 4. Wait for an error or a shutdown signal
	select {
	case err := <-errChan:
		return "Server error", err

	case <-ctx.Done():
		log.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), ac.config.Server.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return "Server forced to shutdown", err
		}
		return "Server exiting gracefully!", nil
	}

}

func (ac *Application) RunTLS(ctx context.Context) (string, error) {
	// 1. Start server with shut down gracefully
	srv := &http.Server{
		Addr:    ":" + ac.config.Server.Port,
		Handler: ac.route,

		ReadTimeout:       ac.config.Server.ReadTimeout,
		ReadHeaderTimeout: ac.config.Server.ReadHeaderTimeout,
		WriteTimeout:      ac.config.Server.WriteTimeout,
		IdleTimeout:       ac.config.Server.IdleTimeout,

		MaxHeaderBytes: ac.config.Server.MaxHeaderBytes,
	}

	// 2. Create a channel to listen for server errors
	errChan := make(chan error, 1)

	pathCert := utils.GetEnv("PATH_CERT_API_GATEWAY", "")
	pathKey := utils.GetEnv("PATH_KEY_API_GATEWAY", "")

	// 3. Listen and serve in a goroutine
	go func() {
		log.Printf("HTTPS Server is running on port %s...", srv.Addr)
		if err := srv.ListenAndServeTLS(pathCert, pathKey); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// 4. Wait for an error or a shutdown signal
	select {
	case err := <-errChan:
		return "Server error", err

	case <-ctx.Done():
		log.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), ac.config.Server.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return "Server forced to shutdown", err
		}
		return "Server exiting gracefully!", nil
	}

}

func StartRedisListener(ctx context.Context, redisClient *redis.Client) {
	pubsub := redisClient.Subscribe(ctx, "image-processing-results")
	ch := pubsub.Channel()

	redisKey := utils.GetEnv("REDIS_KEY_PAYLOAD", "")

	for msg := range ch {
		log.Printf("Received message from Redis: %s\n", msg.Payload)

		token, err := jwt.ParseWithClaims(msg.Payload, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			return []byte(redisKey), nil
		})

		if err != nil {
			log.Printf("Failed to parse JWT: %v", err)
			continue
		}

		if !token.Valid {
			log.Printf("Invalid JWT token: %s", msg.Payload)
			continue
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			log.Printf("Invalid JWT claims")
			continue
		}

		userID, _ := claims["user_id"].(string)
		if userID == "" {
			log.Printf("Missing user_id in JWT")
			continue
		}

		payload := map[string]any{
			"user_id":   userID,
			"status":    claims["status"],
			"file_path": claims["file_path"],
		}

		b, _ := json.Marshal(payload)

		// Tìm đúng User đang kết nối SSE để gửi
		sse.MainBroker.Mu.RLock()

		userChan, ok := sse.MainBroker.Clients[userID]
		if ok {
			log.Printf("Dispatching SSE to user %s", userID)
			userChan <- string(b)
		} else {
			log.Printf("No active SSE client for user %s", userID)
		}

		sse.MainBroker.Mu.RUnlock()
	}
}

// getModuleRoutes is a helper function that takes a slice of Model interfaces
// and returns a slice of routes.Routes by calling the Routes() method on each module
func getModuleRoutes(models []ModelHTTP) []routes.Routes {
	routeList := make([]routes.Routes, len(models))
	for i, model := range models {
		routeList[i] = model.Routes()
	}
	return routeList
}
