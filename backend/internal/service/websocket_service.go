package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	chat_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/chat"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/interceptor"
	"github.com/redis/go-redis/v9"
)

type WebsocketService struct {
	rdb        *redis.Client
	chatClient *client.ChatClient
}

func NewWebsocketService(rdb *redis.Client, chatClient *client.ChatClient) *WebsocketService {
	return &WebsocketService{
		rdb:        rdb,
		chatClient: chatClient,
	}
}

func (s *WebsocketService) CheckMembersInConversation(ctx context.Context, conversationID string, userID int32) (bool, error) {
	data, err := s.rdb.Get(ctx, fmt.Sprintf("conversation:%s:members", conversationID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			result, err := s.chatClient.Client.GetConversationMembers(interceptor.WithUserIDMetadata(ctx, userID), &chat_proto.GetConversationMembersRequest{
				ConversationId: conversationID,
				UserId:         userID,
			})
			if err != nil {
				return false, err
			}

			members := make(map[int32]bool, len(result.UserIds))
			for _, id := range result.UserIds {
				members[id] = true
			}

			membersByte, err := json.Marshal(members)
			if err != nil {
				return false, err
			}

			// Cache the members in Redis
			err = s.rdb.Set(ctx, fmt.Sprintf("conversation:%s:members", conversationID), membersByte, 0).Err()
			if err != nil {
				log.Printf("Failed to cache members in Redis: %v", err)
			}

			_, exists := members[userID]
			return exists, nil
		}

		log.Printf("Failed to get members from Redis: %v", err)

		return false, err
	} else {
		members := make(map[int32]bool)

		err = json.Unmarshal(data, &members)
		if err != nil {
			log.Printf("Failed to unmarshal members from Redis: %v", err)
			return false, err
		}

		_, exists := members[userID]
		return exists, nil
	}
}

func (s *WebsocketService) SendMessage(ctx context.Context, req *chat_proto.SendMessageRequest) (bool, error) {
	result, err := s.chatClient.Client.SendMessage(interceptor.WithUserIDMetadata(ctx, req.SenderId), req)
	if err != nil {
		return false, err
	}

	return result.Success, nil
}
