package routes

import (
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/handler"
	"github.com/gin-gonic/gin"
)

type ChatRoutes struct {
	chat_handler *handler.ChatHandler
}

func NewChatRoutes(handler *handler.ChatHandler) Routes {
	return &ChatRoutes{
		chat_handler: handler,
	}
}

func (cr *ChatRoutes) Register(r *gin.RouterGroup) {
	// chat := r.Group("/chat")
	// {
	// 	// chat.POST("/private-conversation", cr.chat_handler.CreatePrivateConversation)
	// }
}
