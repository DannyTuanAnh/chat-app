package ws

import (
	"github.com/coder/websocket"
)

type Client struct {
	DeviceID string
	UserID   int32
	Conn     *websocket.Conn
}
