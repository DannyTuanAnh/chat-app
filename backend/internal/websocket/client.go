package ws

import (
	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type Client struct {
	DeviceID uuid.UUID
	UserID   uuid.UUID
	Conn     *websocket.Conn
}
