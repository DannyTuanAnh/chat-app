package service

import (
	"fmt"

	"buf.build/go/protovalidate"
	chat_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/chat"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/repository"
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
