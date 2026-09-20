package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/app"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db"
	redis_memory "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/redis"
	"github.com/redis/go-redis/v9"
)

func main() {
	log.Println("Start app")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var authDB db.AuthDB

	// --- Init DB
	log.Println("Init DB...")
	authDB, err := db.InitAuthDB()
	if err != nil {
		log.Fatalf("Database init failed: %v", err)
	}
	defer authDB.Close()
	log.Println("Database connected")

	// --- Init Redis
	log.Println("Init Redis...")
	rdb, err := redis_memory.InitRedis()
	if err != nil {
		log.Fatalf("Redis init failed: %v", err)
	}
	defer rdb.CloseRedis()
	log.Println("Redis connected")

	// log.Println("Start Redis listener...")
	// go app.StartRedisListener(ctx, rdb.Redis_GCP)

	log.Println("Create app...")

	var redisClient *redis.Client

	//local test
	if rdb != nil {
		redisClient = rdb.RDB
	}

	// //deploy
	// if rdb != nil {
	// 	redisClient = rdb.Redis_GCP
	// }

	application := app.NewApplication(ctx, authDB.DB, redisClient)

	log.Println("Run server...")

	var msg string
	if os.Getenv("ENV") == "development" {
		msg, err = application.RunTLS(ctx)
	} else {
		msg, err = application.Run(ctx)
	}

	// Chỉ log lỗi, không kill container
	if err != nil {
		log.Printf("Server error: %s: %v", msg, err)
		// select {} // giữ process sống để Cloud Run không kill ngay
	}

	log.Println(msg)
}

// func retryDB(ctx context.Context) {
// 	ticker := time.NewTicker(10 * time.Second)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-ticker.C:
// 			log.Println("Retrying DB connection...")

// 			if err := db.InitDB(); err != nil {
// 				log.Printf("DB retry failed: %v", err)
// 				continue
// 			}

// 			log.Println("DB reconnected successfully")
// 			return
// 		}
// 	}
// }

// func retryRedis(ctx context.Context) {
// 	ticker := time.NewTicker(10 * time.Second)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-ticker.C:
// 			log.Println("Retrying Redis connection...")

// 			rdb, err := redis_memory.InitRedis()
// 			if err != nil {
// 				log.Printf("Redis retry failed: %v", err)
// 				continue
// 			}

// 			log.Println("Redis reconnected successfully")
// 			go app.StartRedisListener(ctx, rdb.Redis_GCP)
// 			return
// 		}
// 	}
// }
