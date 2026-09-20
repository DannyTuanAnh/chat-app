package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db"
	redis_memory "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/redis"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/server"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Initialize original context for the application
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Init DB
	log.Println("Init DB...")
	authDB, err := db.InitAuthDB()
	if err != nil {
		log.Fatalf("Database init failed: %v", err)
	}
	defer authDB.Close()
	log.Println("Database connected")

	// Init Redis
	log.Println("Init Redis...")
	rdb, err := redis_memory.InitRedis()
	if err != nil {
		log.Fatalf("Redis init failed: %v", err)
	}
	defer rdb.CloseRedis()
	log.Println("Redis connected")

	var redisClient *redis.Client

	//local test
	if rdb != nil {
		redisClient = rdb.RDB
	}

	// //deploy
	// if rdb != nil {
	// 	redisClient = rdb.Redis_GCP
	// }

	websocket := server.NewWebSocketServer(ctx, authDB.DB, redisClient)

	log.Println("Starting Websocket server...")
	msg, err := websocket.RunTLS(ctx)

	if err != nil {
		log.Printf("Websocket server error: %s: %v", msg, err)
	}

	log.Println(msg)
}
