package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/dto"
	notify_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/notify"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/interceptor"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/middleware"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
	"github.com/gin-gonic/gin"
)

type NotifyHandler struct {
	notify_client *client.NotifyClient
}

func NewNotifyHandler(notify_client *client.NotifyClient) *NotifyHandler {
	return &NotifyHandler{
		notify_client: notify_client,
	}
}

func (nh *NotifyHandler) RegisterFCM(ctx *gin.Context) {
	var req dto.RegisterDeviceRequest
	log.Println("RegisterFCM request:", req)
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	// comment tạm thời để test
	// userID, deviceID, valid := nh.validateCtx(ctx)
	// if !valid {
	// 	return
	// }

	// result, err := nh.notify_client.Client.RegisterDevice(interceptor.WithUserIDMetadata(ctx.Request.Context(), userID), &notify_proto.RegisterDeviceRequest{
	// 	DeviceId:  deviceID,
	// 	PushToken: req.PushToken,
	// })

	result, err := nh.notify_client.Client.RegisterDevice(interceptor.WithUserIDMetadata(ctx.Request.Context(), req.UserID), &notify_proto.RegisterDeviceRequest{
		DeviceId:  req.DeviceID,
		PushToken: req.PushToken,
	})

	if err != nil {
		utils.WriteGRPCErrorToGin(ctx, err)
		return
	}

	if !result.Success {
		utils.ResponseErrorAbort(ctx, utils.NewError("Failed to register device", utils.ErrCodeInternal))
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated)
}

func (h *NotifyHandler) validateCtx(ctx *gin.Context) (int32, string, bool) {
	currentUserID, exist := ctx.Get(middleware.CTX_USER_ID_KEY)
	if !exist {
		log.Println("User ID not found in context")
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID not found in context", utils.ErrCodeNotFound))
		return 0, "", false
	}

	currentUserIDInt, ok := currentUserID.(int32)
	if !ok {
		log.Println("User ID in context has invalid type")
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID in context has invalid type", utils.ErrCodeInternal))
		return 0, "", false
	}

	if currentUserIDInt <= 0 {
		log.Println("User ID must be greater than 0")
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(errors.New("UserID must greater than 0")))
		return 0, "", false
	}

	deviceID, exist := ctx.Get(middleware.CTX_DEVICE_ID_KEY)
	if !exist {
		log.Println("Device ID not found in context")
		utils.ResponseErrorAbort(ctx, utils.NewError("Device ID not found in context", utils.ErrCodeNotFound))
		return 0, "", false
	}

	deviceIDStr, ok := deviceID.(string)
	if !ok {
		log.Println("Device ID in context has invalid type")
		utils.ResponseErrorAbort(ctx, utils.NewError("Device ID in context has invalid type", utils.ErrCodeInternal))
		return 0, "", false
	}

	return currentUserIDInt, deviceIDStr, true
}
