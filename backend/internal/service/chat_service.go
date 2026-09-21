package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"buf.build/go/protovalidate"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/chat"
	chat_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/chat"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/repository"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
	ws "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/websocket"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type chatService struct {
	chat_proto.UnimplementedChatServiceServer
	chat_repo      repository.ChatRepository
	validator      protovalidate.Validator
	rdb            *redis.Client
	ctxChatService context.Context
}

func NewChatService(chat_repo repository.ChatRepository, rdb *redis.Client, ctx context.Context) *chatService {
	v, err := protovalidate.New()
	if err != nil {
		panic(fmt.Sprintf("Failed to create validator: %v", err))
	}

	return &chatService{
		chat_repo:      chat_repo,
		validator:      v,
		rdb:            rdb,
		ctxChatService: ctx,
	}
}

func (cs *chatService) CreatePrivateConversation(ctx context.Context, req *chat_proto.CreatePrivateConversationRequest) (*chat_proto.CreatePrivateConversationResponse, error) {
	if err := cs.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	tx, err := cs.chat_repo.BeginTransaction(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to begin transaction: %v", err)
	}
	defer cs.chat_repo.RollBack(ctx, tx, &err)

	conversationID, err := cs.chat_repo.CreateConversation(ctx, tx, sqlc.ConversationTypePrivate)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create conversation: %v", err)
	}

	err = cs.chat_repo.AddMembersToConversation(ctx, tx, conversationID, []int32{req.UserId_1, req.UserId_2})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to add members to conversation: %v", err)
	}

	createSystemMessageParams := sqlc.CreateSystemMessageParams{
		ConversationID: conversationID,
		EventType:      sqlc.SystemEventTypeChatCreated,
		ActorID:        utils.ConvertToPgTypeIntPtr(nil),
		TargetID:       utils.ConvertToPgTypeIntPtr(nil),
		Content:        utils.ConvertToPgTypeText("You are now friends! Start chatting."),
	}

	_, err = cs.chat_repo.CreateSystemMessage(ctx, tx, createSystemMessageParams)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create system message: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to commit transaction: %v", err)
	}

	members := []int32{req.UserId_1, req.UserId_2}

	membersByte, err := json.Marshal([]int32{req.UserId_1, req.UserId_2})
	if err != nil {
		log.Printf("Failed to marshal conversation members: %v\n", err)
	}

	if err := cs.rdb.SetNX(ctx, fmt.Sprintf("conversation:%d:members", conversationID), membersByte, 0).Err(); err != nil {
		log.Printf("Failed to set conversation members in Redis: %v\n", err)
	}

	go cs.systemNotifyViaRedis(cs.ctxChatService, members, conversationID)

	return &chat_proto.CreatePrivateConversationResponse{
		Success: true,
	}, nil
}

func (cs *chatService) systemNotifyViaRedis(ctx context.Context, userIDs []int32, conversationID uuid.UUID) {
	event := ws.RealtimeEvent{
		Event:          "friend_request_accepted",
		UserIDs:        userIDs,
		ConversationID: conversationID.String(),
		Message:        "You are now friends! Start chatting.",
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal event to JSON: %v", err)
		return
	}

	err = cs.rdb.Publish(ctx, "chat_notifications", data).Err()
	if err != nil {
		log.Printf("Failed to publish message to Redis: %v", err)
		return
	}

	log.Printf("Published message to Redis channel 'friend_request_accepted' for users %v", userIDs)
}
