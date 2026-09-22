package ws

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	PING_INTERVAL = 25 * time.Second
	PONG_TIMEOUT  = 10 * time.Second
)

type MessageHandler func(
	client *Client,
	messageType websocket.MessageType,
	data []byte,
)

type OutboundMessage struct {
	Data        []byte
	MessageType websocket.MessageType
}

type Client struct {
	DeviceID string
	UserID   int32

	Conn *websocket.Conn

	SendChan chan OutboundMessage

	Ctx    context.Context
	Cancel context.CancelFunc

	CloseOnce sync.Once
}

func (client *Client) Close() {
	client.CloseOnce.Do(func() {
		log.Printf("Closing connection for user %d, device %s", client.UserID, client.DeviceID)

		if client.Cancel != nil {
			client.Cancel()
		}

		_ = client.Conn.Close(websocket.StatusNormalClosure, "Closing connection")
	})
}

func (client *Client) ReadPump(handler MessageHandler) {
	for {
		select {
		case <-client.Ctx.Done():
			log.Println("ReadPump context done, closing connection")
			client.Close()
			return
		default:
			messageType, data, err := client.Conn.Read(client.Ctx)
			if err != nil {
				log.Println("Error reading from websocket:", err)
				client.Close()
				return
			}

			log.Printf("Received message from user %d, device %s: %s", client.UserID, client.DeviceID, string(data))
			handler(client, messageType, data)
		}
	}

}

func (client *Client) WritePump() {
	for {
		select {
		case <-client.Ctx.Done():
			log.Printf("WritePump stopped: user=%d device=%s", client.UserID, client.DeviceID)
			return
		case message, ok := <-client.SendChan:
			if !ok {
				log.Printf("SendChan closed: user=%d device=%s", client.UserID, client.DeviceID)
				return
			}

			err := client.Conn.Write(client.Ctx, message.MessageType, message.Data)
			if err != nil {
				log.Printf("Websocket write error: user=%d device=%s error=%v", client.UserID, client.DeviceID, err)
				client.Close()
				return
			}

			log.Println("Sent message to user", client.UserID, "device", client.DeviceID, "message:", string(message.Data))
		}
	}
}

func (client *Client) Heartbeat() {
	ticker := time.NewTicker(PING_INTERVAL)
	defer ticker.Stop()

	for {
		select {
		case <-client.Ctx.Done():
			return
		case <-ticker.C:
			pingCtx, pingCancel := context.WithTimeout(client.Ctx, PONG_TIMEOUT)

			err := client.Conn.Ping(pingCtx)
			pingCancel()

			if err != nil {
				log.Printf("heartbeat failed: user=%d device=%s error=%v", client.UserID, client.DeviceID, err)
				client.Close()
				return
			}

			log.Println("heartbeat: pong received")
		}
	}
}
