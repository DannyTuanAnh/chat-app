package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/config"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/notify"

	"github.com/jackc/pgx/v5/pgxpool"
)

type NotifyDB struct {
	DB     *sqlc.Queries
	DBPool *pgxpool.Pool
}

// local test
func InitNotifyDB() (NotifyDB, error) {
	connStr := config.NewConfigNotifyDB().DB_DNS()

	conf, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return NotifyDB{}, fmt.Errorf("error parsing DB config: %w", err)
	}

	conf.MaxConns = 50
	conf.MinConns = 5
	conf.MaxConnLifetime = 30 * time.Minute
	conf.MaxConnIdleTime = 5 * time.Minute
	conf.HealthCheckPeriod = 1 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, conf)
	if err != nil {
		return NotifyDB{}, fmt.Errorf("error creating DB pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close() // Đóng pool nếu ping thất bại
		return NotifyDB{}, fmt.Errorf("error pinging DB: %w", err)
	}

	// Gán pool và khởi tạo sqlc.Queries
	return NotifyDB{
		DBPool: pool,
		DB:     sqlc.New(pool),
	}, nil
}

// Close đóng connection pool (gọi khi shutdown app)
func (db *NotifyDB) Close() {
	if db.DBPool != nil {
		db.DBPool.Close()
		log.Println("Database connection closed")
	}
}
