package handler

import "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"

type ChatHandler struct {
	chat_client *client.ChatClient
}

func NewChatHandler(chat_client *client.ChatClient) *ChatHandler {
	return &ChatHandler{
		chat_client: chat_client,
	}
}
