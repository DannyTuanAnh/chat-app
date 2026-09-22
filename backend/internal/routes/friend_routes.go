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
		friend.GET("/list", fr.friend_handler.GetFriendList)

		friendRequest := friend.Group("/request")
		{
			friendRequest.POST("/send", fr.friend_handler.SendFriendRequest)
			friendRequest.GET("/pending", fr.friend_handler.GetPendingFriendRequest)
			friendRequest.GET("/sent", fr.friend_handler.GetSentFriendRequest)
			friendRequest.POST("/reject", fr.friend_handler.RejectFriendRequest)
			friendRequest.POST("/accept", fr.friend_handler.AcceptFriendRequest)
		}
	}
}
