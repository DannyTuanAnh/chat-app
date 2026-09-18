package ws

import (
	"context"
	"log"
	"time"

	"github.com/coder/websocket"
)

const (
	pingInterval = 25 * time.Second
	pongTimeout  = 10 * time.Second
)

func Heartbeat(ctx context.Context, cancelConnection context.CancelFunc, conn *websocket.Conn) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, pingCancel := context.WithTimeout(ctx, pongTimeout)

			err := conn.Ping(pingCtx)
			pingCancel()

			if err != nil {
				log.Printf("heartbeat failed: %v", err)
				cancelConnection()
				return
			}

			log.Println("heartbeat: pong received")
		}
	}
}
