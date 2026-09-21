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
)

func main() {
	// Initialize original context for the application
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 2. Initialize database connection
	chatDB, err := db.InitChatDB()

	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
		return
	}
	defer chatDB.Close()

	// --- Init Redis
	log.Println("Init Redis...")
	rdb, err := redis_memory.InitRedis()
	if err != nil {
		log.Fatalf("Redis init failed: %v", err)
	}
	defer rdb.CloseRedis()
	log.Println("Redis connected")

	// 3. Initialize application
	chatServer, err := server.NewChatServer(ctx, chatDB, rdb.RDB)
	if err != nil {
		log.Fatalf("Failed to initialize user server: %v", err)
		return
	}

	// 4. Run the application and capture any error message
	msg, err := chatServer.Run()

	if err != nil {
		log.Fatalf("%s: %v\n", msg, err)
	}

	log.Println(msg)
}
