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
	notifyDB, err := db.InitNotifyDB()

	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
		return
	}
	defer notifyDB.Close()

	// 3. Initialize Redis connection
	rdb, err := redis_memory.InitRedis()

	if err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
		return
	}
	defer rdb.CloseRedis()

	// 3. Initialize application
	notifyServer, err := server.NewNotifyServer(ctx, notifyDB, rdb.RDB)
	if err != nil {
		log.Fatalf("Failed to initialize notify server: %v", err)
		return
	}

	// 4. Run the application and capture any error message
	msg, err := notifyServer.Run()

	if err != nil {
		log.Fatalf("%s: %v\n", msg, err)
	}

	log.Println(msg)
}
