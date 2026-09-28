package ws

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"
)

type NewRealtimeEvent struct {
	Event          string     `json:"event_type"`
	FromUserID     int32      `json:"from_user_id"`
	FromDeviceID   string     `json:"from_device_id"`
	ToUserIDs      []int32    `json:"to_user_ids"`
	ConversationID string     `json:"conversation_id"`
	Message        NewContent `json:"message"`
	SentAt         time.Time  `json:"sent_at"`
}

type NewContent struct {
	FromUserID    int32  `json:"from_user_id"`
	Message       string `json:"message"`
	SystemMessage bool   `json:"system_message"`
}

type NewWSResponse struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

type NewManager struct {
	Clients map[int32]map[string]*Client
	mu      sync.RWMutex
}

func NewNewManager() *NewManager {
	return &NewManager{
		Clients: make(map[int32]map[string]*Client),
	}
}

func (cm *NewManager) AddClient(client *Client) {
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
func (cm *NewManager) RemoveClient(client *Client) bool {
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

func (cm *NewManager) CloseAll() {
	cm.mu.Lock()

	clients := make([]*Client, 0, len(cm.Clients))

	for _, devices := range cm.Clients {
		for _, client := range devices {
			clients = append(clients, client)
		}
	}

	cm.mu.Unlock()

	for _, client := range clients {
		client.Close()
	}

	cm.mu.Lock()
	cm.Clients = make(map[int32]map[string]*Client)
	cm.mu.Unlock()
}

func (cm *NewManager) GetClientsByUserID(clientID int32) ([]*Client, bool) {
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

func (cm *NewManager) GetClient(clientID int32, deviceID string) (*Client, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	devices, exists := cm.Clients[clientID]
	if !exists {
		return nil, false
	}

	client, exists := devices[deviceID]
	return client, exists
}

type NewSendToParams struct {
	TargetClientIDs []int32
	CurrentClientID int32
	CurrentDeviceID string
	MessageType     websocket.MessageType
	Data            NewContent
}

func (cm *NewManager) SendTo(ctx context.Context, arg SendToParams) error {
	cm.mu.RLock()

	targetClientDevices := make([]*Client, 0)
	for _, targetClientID := range arg.TargetClientIDs {
		devices, exists := cm.Clients[targetClientID]
		if !exists {
			cm.mu.RUnlock()
			return fmt.Errorf("client with ID %d not found", targetClientID)
		}

		for _, device := range devices {
			targetClientDevices = append(targetClientDevices, device)
		}
	}

	currentClientDevices, currentExists := cm.Clients[arg.CurrentClientID]
	if !currentExists {
		cm.mu.RUnlock()
		return fmt.Errorf("current client with ID %d not found", arg.CurrentClientID)
	}

	recipient := make([]*Client, 0, len(targetClientDevices)+len(currentClientDevices)-1)

	recipient = append(recipient, targetClientDevices...)

	for _, currentClientDevice := range currentClientDevices {
		if currentClientDevice.DeviceID != arg.CurrentDeviceID {
			recipient = append(recipient, currentClientDevice)
		}
	}

	cm.mu.RUnlock()

	for _, client := range recipient {
		message := OutboundMessage{
			Data:        arg.Data,
			MessageType: arg.MessageType,
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while sending to client %d", client.UserID)
		case <-client.Ctx.Done():
			log.Println("Client context done while sending to client", client.UserID, "device", client.DeviceID, "skipping client")
			continue
		case client.SendChan <- message:
			// Successfully sent to client's send channel
		default:
			log.Println("Timeout sending to client", client.UserID, "device", client.DeviceID, "skipping and closing send channel")
			client.Close()
		}
	}

	return nil
}

type NewSendToUsersParams struct {
	UserIDs     []int32
	MessageType websocket.MessageType
	Data        Content
}

func (cm *NewManager) SendToUsers(ctx context.Context, arg NewSendToUsersParams) error {
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

	message := OutboundMessage{
		Data:        arg.Data,
		MessageType: arg.MessageType,
	}

	for _, client := range recipients {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled while sending to users")
		case <-client.Ctx.Done():
			log.Println("Client context done while sending to users, skipping client", client.UserID, "device", client.DeviceID)
			continue
		case client.SendChan <- message:
			// log.Println("Sent message to client", client.UserID, "device", client.DeviceID)
			// Successfully sent to client's send channel
		default:
			log.Println("Timeout sending to client", client.UserID, "device", client.DeviceID, "skipping and closing send channel")
			client.Close()
		}
	}

	return nil
}

type NewBroadcastParams struct {
	CurrentClientID       int32
	CurrentClientDeviceID string
	MessageType           websocket.MessageType
	Data                  NewContent
}

func (cm *NewManager) Broadcast(ctx context.Context, arg BroadcastParams) {
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

	message := OutboundMessage{
		Data:        arg.Data,
		MessageType: arg.MessageType,
	}

	for _, client := range clients {
		select {
		case <-ctx.Done():
			log.Println("Context canceled while broadcasting, stopping broadcast")
			return
		case <-client.Ctx.Done():
			log.Println("Client context done while broadcasting, skipping client", client.UserID, "device", client.DeviceID)
			continue
		case client.SendChan <- message:
			// Successfully sent to client's send channel
		default:
			log.Println("Timeout broadcasting to client", client.UserID, "device", client.DeviceID, "skipping and closing send channel")
			client.Close()
		}
	}
}
