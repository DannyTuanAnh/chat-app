package service

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/chat"
	chat_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/chat"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/repository"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type chatService struct {
	chat_proto.UnimplementedChatServiceServer
	chat_repo repository.ChatRepository
	validator protovalidate.Validator
}

func NewChatService(chat_repo repository.ChatRepository) *chatService {
	v, err := protovalidate.New()
	if err != nil {
		panic(fmt.Sprintf("Failed to create validator: %v", err))
	}

	return &chatService{
		chat_repo: chat_repo,
		validator: v,
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

	return &chat_proto.CreatePrivateConversationResponse{
		Success: true,
	}, nil
}
