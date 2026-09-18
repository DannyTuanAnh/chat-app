package ws

import (
	"context"
	"fmt"
	"sync"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type ClientManager struct {
	Clients map[uuid.UUID]map[uuid.UUID]*Client
	mu      sync.RWMutex
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		Clients: make(map[uuid.UUID]map[uuid.UUID]*Client),
	}
}

func (cm *ClientManager) AddClient(client *Client) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.Clients[client.UserID]; !exists {
		cm.Clients[client.UserID] = make(map[uuid.UUID]*Client)
	}

	cm.Clients[client.UserID][client.DeviceID] = client
}

// flow để xử lý tình huống race condition: khi client bị disconnect
// Nhưng server chưa kịp remove client khỏi map, và client đó lại connect lại với cùng sessionID
// Khi đó, client mới sẽ overwrite client cũ trong map, và client cũ sẽ bị remove khỏi map
// => Việc kiểm tra (currentClient != client) sẽ giúp tránh việc remove client mới khỏi map
func (cm *ClientManager) RemoveClient(client *Client) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	devices, exists := cm.Clients[client.UserID]
	if !exists {
		return false
	}

	currentClient, exists := devices[client.DeviceID]
	if !exists {
		return false
	}

	if currentClient != client {
		return false
	}

	delete(devices, client.DeviceID)

	if len(devices) == 0 {
		delete(cm.Clients, client.UserID)

		return true
	}

	return false
}

func (cm *ClientManager) CloseAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, devices := range cm.Clients {
		for _, client := range devices {
			client.Conn.Close(websocket.StatusNormalClosure, "server shutting down")
		}
	}

	cm.Clients = make(map[uuid.UUID]map[uuid.UUID]*Client)
}

func (cm *ClientManager) GetClient(clientID, deviceID uuid.UUID) (*Client, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	client, exists := cm.Clients[clientID][deviceID]
	return client, exists
}

type SendToParams struct {
	TargetClientID  uuid.UUID
	CurrentClientID uuid.UUID
	CurrentDeviceID uuid.UUID
	MessageType     websocket.MessageType
	Data            []byte
}

func (cm *ClientManager) SendTo(ctx context.Context, arg SendToParams) error {
	cm.mu.RLock()

	targetClientDevices, targetExists := cm.Clients[arg.TargetClientID]
	if !targetExists {
		cm.mu.RUnlock()
		return fmt.Errorf("client with ID %s not found", arg.TargetClientID)
	}

	currentClientDevices, currentExists := cm.Clients[arg.CurrentClientID]
	if !currentExists {
		cm.mu.RUnlock()
		return fmt.Errorf("current client with ID %s not found", arg.CurrentClientID)
	}

	recipient := make([]*Client, 0, len(targetClientDevices)+len(currentClientDevices)-1)

	for _, targetClientDevice := range targetClientDevices {
		recipient = append(recipient, targetClientDevice)
	}

	for _, currentClientDevice := range currentClientDevices {
		if currentClientDevice.DeviceID != arg.CurrentDeviceID {
			recipient = append(recipient, currentClientDevice)
		}
	}

	cm.mu.RUnlock()

	for _, client := range recipient {
		if err := client.Conn.Write(ctx, arg.MessageType, arg.Data); err != nil {
			return fmt.Errorf("error sending message to client %s: %v", client.UserID, err)
		}
	}

	return nil
}

type BroadcastParams struct {
	CurrentClientID       uuid.UUID
	CurrentClientDeviceID uuid.UUID
	MessageType           websocket.MessageType
	Data                  []byte
}

func (cm *ClientManager) Broadcast(ctx context.Context, arg BroadcastParams) {
	cm.mu.RLock()

	clients := make([]*Client, 0, len(cm.Clients))

	for clientID, clientDevices := range cm.Clients {
		if clientID == arg.CurrentClientID {
			for deviceID, client := range clientDevices {
				if deviceID != arg.CurrentClientDeviceID {
					clients = append(clients, client)
				}
			}
		} else {
			for _, client := range clientDevices {
				clients = append(clients, client)
			}
		}
	}

	cm.mu.RUnlock()

	for _, client := range clients {
		if err := client.Conn.Write(ctx, arg.MessageType, arg.Data); err != nil {
			fmt.Printf("Error broadcasting to client %s: %v\n", client.UserID, err)
		}
	}
}
