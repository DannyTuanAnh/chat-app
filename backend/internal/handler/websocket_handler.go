package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/middleware"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/service"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
	ws "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/websocket"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

var lastSeen = make(map[int32]time.Time)

type WebSocketHandler struct {
	userService *service.WebsocketService
	manager     *ws.ClientManager
	lastSeen    map[int32]time.Time
}

type Content struct {
	To      int32  `json:"to"`
	Message string `json:"message"`
}

func NewWebSocketHandler(userService *service.WebsocketService, manager *ws.ClientManager) *WebSocketHandler {
	lastSeen = make(map[int32]time.Time)

	return &WebSocketHandler{
		userService: userService,
		manager:     manager,
		lastSeen:    lastSeen,
	}
}

func (h *WebSocketHandler) HandleWebsocket(ctx *gin.Context) {
	currentUserID, deviceID, valid := h.validateWebsocket(ctx)
	if !valid {
		return
	}

	conn, err := websocket.Accept(ctx.Writer, ctx.Request, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"}, // Dấu * cho phép mọi trang web kết nối đến để test
	})
	if err != nil {
		utils.ResponseErrorAbort(ctx, utils.NewError("Failed to accept websocket connection", utils.ErrCodeInternal))
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "Closing connection")

	client := &ws.Client{
		UserID:   currentUserID,
		DeviceID: deviceID,
		Conn:     conn,
	}

	h.manager.AddClient(client)

	defer func() {
		becomeOffline := h.manager.RemoveClient(client)

		if becomeOffline {
			if off := h.manager.IsUserOffline(currentUserID); off {
				lastSeen[currentUserID] = time.Now()
				log.Println("User", currentUserID, "is now offline. Last seen at:", lastSeen[currentUserID])
			}
		}
	}()

	log.Println("User ", currentUserID, "connected with device", deviceID)

	c, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ws.Heartbeat(c, cancel, conn)

	for {
		messageType, data, err := conn.Read(c)
		if err != nil {
			log.Printf("WS READ ERROR: user_id=%d remote=%s error=%v", currentUserID, ctx.Request.RemoteAddr, err)
			return
		}

		log.Printf("WS MESSAGE: remote=%s type=%v message=%s", ctx.Request.RemoteAddr, messageType, data)

		var content Content

		if err := json.Unmarshal(data, &content); err != nil {
			log.Printf("WS UNMARSHAL ERROR: remote=%s error=%v", ctx.Request.RemoteAddr, err)
			continue
		}

		if content.To == 0 {
			argBroadCast := ws.BroadcastParams{
				CurrentClientID:       client.UserID,
				CurrentClientDeviceID: client.DeviceID,
				MessageType:           messageType,
				Data:                  []byte(content.Message),
			}

			h.manager.Broadcast(c, argBroadCast)
			continue
		}

		argSendTo := ws.SendToParams{
			TargetClientID:  content.To,
			CurrentClientID: client.UserID,
			CurrentDeviceID: client.DeviceID,
			MessageType:     messageType,
			Data:            []byte(content.Message),
		}

		if err = h.manager.SendTo(c, argSendTo); err != nil {
			log.Printf("WS SEND ERROR: remote=%s error=%v", ctx.Request.RemoteAddr, err)
		}
	}
}

func (h *WebSocketHandler) validateWebsocket(ctx *gin.Context) (int32, string, bool) {
	currentUserID, exist := ctx.Get(middleware.CTX_USER_ID_KEY)
	if !exist {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID not found in context", utils.ErrCodeNotFound))
		return 0, "", false
	}

	currentUserIDInt, ok := currentUserID.(int32)
	if !ok {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID in context has invalid type", utils.ErrCodeInternal))
		return 0, "", false
	}

	if currentUserIDInt <= 0 {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(errors.New("UserID must greater than 0")))
		return 0, "", false
	}

	deviceID, exist := ctx.Get(middleware.CTX_DEVICE_ID_KEY)
	if !exist {
		utils.ResponseErrorAbort(ctx, utils.NewError("Device ID not found in context", utils.ErrCodeNotFound))
		return 0, "", false
	}

	deviceIDStr, ok := deviceID.(string)
	if !ok {
		utils.ResponseErrorAbort(ctx, utils.NewError("Device ID in context has invalid type", utils.ErrCodeInternal))
		return 0, "", false
	}

	return currentUserIDInt, deviceIDStr, true
}
