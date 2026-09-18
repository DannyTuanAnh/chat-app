package handler

import (
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/service"
	"github.com/gin-gonic/gin"
)

type WebSocketHandler struct {
	userService *service.WebsocketService
}

func NewWebSocketHandler(userService *service.WebsocketService) *WebSocketHandler {
	return &WebSocketHandler{
		userService: userService,
	}
}

func (h *WebSocketHandler) HandleWebsocket(ctx *gin.Context) {}
