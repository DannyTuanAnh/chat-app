package routes

import (
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/handler"
	"github.com/gin-gonic/gin"
)

type NotifyRoutes struct {
	notify_handler *handler.NotifyHandler
}

func NewNotifyRoutes(handler *handler.NotifyHandler) Routes {
	return &NotifyRoutes{
		notify_handler: handler,
	}
}

func (ur *NotifyRoutes) Register(r *gin.RouterGroup) {
	// notify := r.Group("/notify")
	// {
	// 	notify.GET("/sse", ur.notify_handler.HandleSSE)
	// }
}
