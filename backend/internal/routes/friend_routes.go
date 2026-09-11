package routes

import (
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/handler"
	"github.com/gin-gonic/gin"
)

type FriendRoutes struct {
	friend_handler *handler.FriendHandler
}

func NewFriendRoutes(handler *handler.FriendHandler) Routes {
	return &FriendRoutes{
		friend_handler: handler,
	}
}

func (fr *FriendRoutes) Register(r *gin.RouterGroup) {
	friend := r.Group("/friend")
	{
		friend.POST("/friend-request", fr.friend_handler.SendFriendRequest)
		friend.GET("/friend-request/pending", fr.friend_handler.GetPendingFriendRequest)
		friend.GET("/friend-request/sent", fr.friend_handler.GetSentFriendRequest)
		friend.POST("/friend-request/reject", fr.friend_handler.RejectFriendRequest)
	}
}
