package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/middleware"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/sse"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
	"github.com/gin-gonic/gin"
)

type NotifyHandler struct {
	user_client *client.UserClient
}

func NewNotifyHandler(user_client *client.UserClient) *NotifyHandler {
	return &NotifyHandler{
		user_client: user_client,
	}
}

func (n *NotifyHandler) HandleSSE(ctx *gin.Context) {
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")
	ctx.Writer.Header().Set("X-Accel-Buffering", "no")

	userId, exist := ctx.Get(middleware.CTX_USER_ID_KEY)
	log.Println("User ID from context:", userId, "Exist:", exist)
	if !exist {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID not found in context", utils.ErrCodeNotFound))
	}

	userID, ok := userId.(int32)
	if !ok {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID in context has invalid type", utils.ErrCodeInternal))
		return
	}

	if userID <= 0 {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(errors.New("UserID must greater than 0")))
		return
	}

	ctx.Writer.WriteHeader(http.StatusOK)

	// 3. Force first body chunk immediately
	_, err := ctx.Writer.Write([]byte(": connected\n\n"))
	if err != nil {
		log.Println("Initial SSE write error:", err)
		return
	}
	ctx.Writer.Flush()

	userIDStr := strconv.FormatInt(int64(userID), 10)
	messageChan := sse.MainBroker.AddClient(userIDStr)
	defer sse.MainBroker.RemoveClient(userIDStr)

	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case message := <-messageChan:
			// Send the message to the client
			log.Println("Sending message to client:", message)
			_, err := ctx.Writer.Write([]byte("data: " + message + "\n\n"))
			if err != nil {
				log.Println("SSE message write error:", err)
				return
			}
			ctx.Writer.Flush()

		case <-heartbeat.C:
			log.Println("Sending SSE heartbeat")

			_, err := ctx.Writer.Write([]byte(": ping\n\n"))
			if err != nil {
				log.Println("Heartbeat write error:", err)
				return
			}
			ctx.Writer.Flush()

		case <-ctx.Request.Context().Done():
			log.Println("SSE client disconnected:", userIDStr)
			return
		}
	}
}
