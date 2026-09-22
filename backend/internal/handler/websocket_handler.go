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

type Content struct {
	To      int32  `json:"to"`
	Message string `json:"message"`
}

type WebSocketHandler struct {
	userService *service.WebsocketService
	manager     *ws.ClientManager
	lastSeen    map[int32]time.Time
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
	log.Println("HandleWebsocket called")
	currentUserID, deviceID, valid := h.validateWebsocket(ctx)
	if !valid {
		return
	}

	conn, err := websocket.Accept(ctx.Writer, ctx.Request, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"}, // Dấu * cho phép mọi trang web kết nối đến để test
	})
	if err != nil {
		log.Println("Failed to accept websocket connection:", err)
		utils.ResponseErrorAbort(ctx, utils.NewError("Failed to accept websocket connection", utils.ErrCodeInternal))
		return
	}

	clientCtx, cancel := context.WithCancel(context.Background())

	client := &ws.Client{
		UserID:   currentUserID,
		DeviceID: deviceID,
		Conn:     conn,

		SendChan: make(chan ws.OutboundMessage, 256), // Buffer size of 256 messages

		Ctx:    clientCtx,
		Cancel: cancel,
	}

	h.manager.AddClient(client)

	defer func() {
		client.Close()

		becomeOffline := h.manager.RemoveClient(client)

		if becomeOffline {

			lastSeen[currentUserID] = time.Now()
			log.Println("User", currentUserID, "is now offline. Last seen at:", lastSeen[currentUserID])

		}
	}()

	log.Println("User", currentUserID, "connected with device", deviceID)

	go client.WritePump()

	go client.Heartbeat()

	client.ReadPump(h.handleClientMessage)
}

func (h *WebSocketHandler) handleClientMessage(client *ws.Client, messageType websocket.MessageType, data []byte) {
	var content Content

	if err := json.Unmarshal(data, &content); err != nil {
		log.Printf("WS UNMARSHAL ERROR: user=%d device=%s error=%v", client.UserID, client.DeviceID, err)
		return
	}

	if content.To == 0 {

		argBroadCast := ws.BroadcastParams{
			CurrentClientID:       client.UserID,
			CurrentClientDeviceID: client.DeviceID,
			MessageType:           messageType,
			Data:                  []byte(content.Message),
		}

		h.manager.Broadcast(client.Ctx, argBroadCast)

		return
	}

	argSendTo := ws.SendToParams{
		TargetClientID:  content.To,
		CurrentClientID: client.UserID,
		CurrentDeviceID: client.DeviceID,
		MessageType:     messageType,
		Data:            []byte(content.Message),
	}

	err := h.manager.SendTo(client.Ctx, argSendTo)

	if err != nil {
		log.Printf("WS SEND ERROR: user=%d error=%v", client.UserID, err)
	}
}

func (h *WebSocketHandler) validateWebsocket(ctx *gin.Context) (int32, string, bool) {
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
