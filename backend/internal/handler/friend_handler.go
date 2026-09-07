package handler

import (
	"errors"
	"net/http"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/dto"
	friend_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/friend"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/interceptor"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/middleware"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
	"github.com/gin-gonic/gin"
)

type FriendHandler struct {
	friend_client *client.FriendClient
}

func NewFriendHandler(friend_client *client.FriendClient) *FriendHandler {
	return &FriendHandler{
		friend_client: friend_client,
	}
}

func (fh *FriendHandler) SendFriendRequest(ctx *gin.Context) {
	var request dto.SendFriendRequestRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	currentUserID, exist := ctx.Get(middleware.CTX_USER_ID_KEY)
	if !exist {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID not found in context", utils.ErrCodeNotFound))
		return
	}

	currentUserIDInt, ok := currentUserID.(int32)
	if !ok {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID in context has invalid type", utils.ErrCodeInternal))
		return
	}

	if currentUserIDInt <= 0 {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(errors.New("UserID must greater than 0")))
		return
	}

	arg := friend_proto.SendFriendRequestRequest{
		CurrentUserId: currentUserIDInt,
		TargetUserId:  request.TargetUserID,
	}

	_, err := fh.friend_client.Client.SendFriendRequest(interceptor.WithUserIDMetadata(ctx.Request.Context(), currentUserIDInt), &arg)
	if err != nil {
		utils.WriteGRPCErrorToGin(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated)
}
