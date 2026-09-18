package service

import "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"

type WebsocketService struct {
	userClient *client.UserClient
}

func NewWebsocketService(userClient *client.UserClient) *WebsocketService {
	return &WebsocketService{
		userClient: userClient,
	}
}
