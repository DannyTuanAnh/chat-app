package ws

import (
	"context"
	"fmt"
	"sync"

	"github.com/coder/websocket"
)

type OldClientManager struct {
	Clients map[int32]map[string]*Client
	mu      sync.RWMutex
}

func NewOldClientManager() *OldClientManager {
	return &OldClientManager{
		Clients: make(map[int32]map[string]*Client),
	}
}

func (cm *OldClientManager) AddClient(client *Client) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.Clients[client.UserID]; !exists {
		cm.Clients[client.UserID] = make(map[string]*Client)
	}

	cm.Clients[client.UserID][client.DeviceID] = client
}

// flow để xử lý tình huống race condition: khi client bị disconnect
// Nhưng server chưa kịp remove client khỏi map, và client đó lại connect lại với cùng sessionID
// Khi đó, client mới sẽ overwrite client cũ trong map, và client cũ sẽ bị remove khỏi map
// => Việc kiểm tra (currentClient != client) sẽ giúp tránh việc remove client mới khỏi map
func (cm *OldClientManager) RemoveClient(client *Client) bool {
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

func (cm *OldClientManager) IsUserOffline(userID int32) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	devices, exists := cm.Clients[userID]
	if !exists || len(devices) == 0 {
		return true
	}

	return false
}

// func (cm *OldClientManager) CloseAll() {
// 	cm.mu.Lock()
// 	defer cm.mu.Unlock()

// 	for _, devices := range cm.Clients {
// 		for _, client := range devices {
// 			client.Conn.Close(websocket.StatusNormalClosure, "server shutting down")
// 		}
// 	}

// 	cm.Clients = make(map[int32]map[string]*Client)
// }

func (cm *OldClientManager) GetClientsByUserID(clientID int32) ([]*Client, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	devices, exists := cm.Clients[clientID]
	if !exists {
		return nil, false
	}

	clients := make([]*Client, 0, len(devices))
	for _, client := range devices {
		clients = append(clients, client)
	}

	return clients, true
}

func (cm *OldClientManager) GetClient(clientID int32, deviceID string) (*Client, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	client, exists := cm.Clients[clientID][deviceID]
	return client, exists
}

type OldSendToParams struct {
	TargetClientID  int32
	CurrentClientID int32
	CurrentDeviceID string
	MessageType     websocket.MessageType
	Data            []byte
}

func (cm *OldClientManager) SendTo(ctx context.Context, arg OldSendToParams) error {
	cm.mu.RLock()

	targetClientDevices, targetExists := cm.Clients[arg.TargetClientID]
	if !targetExists {
		cm.mu.RUnlock()
		return fmt.Errorf("client with ID %d not found", arg.TargetClientID)
	}

	currentClientDevices, currentExists := cm.Clients[arg.CurrentClientID]
	if !currentExists {
		cm.mu.RUnlock()
		return fmt.Errorf("current client with ID %d not found", arg.CurrentClientID)
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
			return fmt.Errorf("error sending message to client %d: %v", client.UserID, err)
		}
	}

	return nil
}

type OldSendToUsersParams struct {
	UserIDs     []int32
	MessageType websocket.MessageType
	Data        Content
}

func (cm *OldClientManager) SendToUsers(ctx context.Context, arg OldSendToUsersParams) error {
	cm.mu.RLock()

	recipients := make([]*Client, 0)

	for _, userID := range arg.UserIDs {
		devices, exists := cm.Clients[userID]
		if !exists {
			continue
		}

		for _, client := range devices {
			recipients = append(recipients, client)
		}
	}

	cm.mu.RUnlock()

	for _, client := range recipients {
		if err := client.Conn.Write(ctx, arg.MessageType, []byte(arg.Data.Message)); err != nil {
			return fmt.Errorf("error sending message to client %d: %v", client.UserID, err)
		}
	}

	return nil
}

type OldBroadcastParams struct {
	CurrentClientID       int32
	CurrentClientDeviceID string
	MessageType           websocket.MessageType
	Data                  []byte
}

func (cm *OldClientManager) Broadcast(ctx context.Context, arg OldBroadcastParams) {
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
			fmt.Printf("Error broadcasting to client %d: %v\n", client.UserID, err)
		}
	}
}
