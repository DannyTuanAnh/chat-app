package service

import (
	"context"
	"fmt"
	"log"

	"buf.build/go/protovalidate"
	"firebase.google.com/go/v4/messaging"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/notify"
	chat_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/chat"
	notify_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/notify"
	user_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/user"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/interceptor"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/repository"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NotifyService struct {
	notify_proto.UnimplementedNotifyServiceServer
	notify_repo repository.NotifyRepository
	validator   protovalidate.Validator

	userClient *client.UserClient
	chatClient *client.ChatClient

	messagingClient *messaging.Client

	rdb *redis.Client
}

func NewNotifyService(notify_repo repository.NotifyRepository, userClient *client.UserClient, chatClient *client.ChatClient, messagingClient *messaging.Client, rdb *redis.Client) *NotifyService {
	v, err := protovalidate.New()
	if err != nil {
		panic(fmt.Sprintf("Failed to create validator: %v", err))
	}

	return &NotifyService{
		notify_repo:     notify_repo,
		validator:       v,
		userClient:      userClient,
		chatClient:      chatClient,
		messagingClient: messagingClient,
		rdb:             rdb,
	}
}

func (ns *NotifyService) RegisterDevice(ctx context.Context, req *notify_proto.RegisterDeviceRequest) (*notify_proto.RegisterDeviceResponse, error) {
	if err := ns.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	userID, ok := ctx.Value(interceptor.UserIDContextKey).(int32)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user ID not found in context")
	}

	deviceID, err := uuid.Parse(req.GetDeviceId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid device ID: %v", err)
	}

	success, err := ns.notify_repo.CreateUserInfo(ctx, sqlc.CreateFCMNotificationParams{
		UserID:     userID,
		DeviceUuid: deviceID,
		PushToken:  req.GetPushToken(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create user info: %v", err)
	}

	if !success {
		return nil, status.Error(codes.Internal, "failed to create user info: no rows affected")
	}

	return &notify_proto.RegisterDeviceResponse{
		Success: true,
	}, nil
}

const (
	ConversationTypePrivate string = "private"
	ConversationTypeGroup   string = "group"
)

func (ns *NotifyService) SendToFID(ctx context.Context, req *notify_proto.SendToFIDRequest) (*notify_proto.SendToFIDResponse, error) {
	if err := ns.validator.Validate(req); err != nil {
		log.Println("Request: ", req)
		log.Println("Validation error in SendToFID:", err)
		return nil, validation.BuildValidationError(err)
	}

	userID, ok := ctx.Value(interceptor.UserIDContextKey).(int32)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user ID not found in context")
	}

	result, err := ns.chatClient.Client.GetConversationType(interceptor.WithUserIDMetadata(ctx, userID), &chat_proto.GetConversationTypeRequest{
		ConversationId: req.GetConversationId(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation type: %v", err)
	}

	var title, avatar_url string

	if result.ConversationType == ConversationTypeGroup {
		groupInfo, err := ns.chatClient.Client.GetGroupInfo(interceptor.WithUserIDMetadata(ctx, userID), &chat_proto.GetGroupInfoRequest{
			ConversationId: req.GetConversationId(),
		})

		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get group info: %v", err)
		}

		title = groupInfo.GroupName
		avatar_url = groupInfo.GroupAvatarUrl

	} else {
		userInfo, err := ns.userClient.Client.GetProfile(interceptor.WithUserIDMetadata(ctx, userID), &user_proto.GetProfileRequest{
			UserId: userID,
		})

		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get user info: %v", err)
		}

		title = *userInfo.Name
		avatar_url = *userInfo.AvatarUrl
	}

	for _, toUserID := range req.GetToUserIds() {
		devices, err := ns.rdb.SMembers(ctx, fmt.Sprintf("ws:presence:user:%d", toUserID)).Result()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get user devices from Redis: %v", err)
		}

		userInfoNotification, err := ns.notify_repo.GetUserInfoByUserID(ctx, toUserID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get user info in notify data: %v", err)
		}

		if len(devices) == 0 {
			arg := SendEachForMultiDevicesRequest{
				UserID:         userID,
				ConversationId: req.GetConversationId(),
				Message:        req.GetMessage(),
			}

			success, err := ns.SendEachForMultiDevices(ctx, arg)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to send message to multiple devices: %v", err)
			}

			if !success {
				return nil, status.Error(codes.Internal, "failed to send message to multiple devices")
			}

		} else if len(devices) == len(userInfoNotification) {
			// User is online on all devices, no need to send push notification
			log.Println("User", toUserID, "is online on all devices, no need to send push notification")
			return &notify_proto.SendToFIDResponse{
				Success: true,
			}, nil

		} else {
			log.Println("User", toUserID, "is online on some devices, sending push notification to offline devices")
			deviceNeedToNotify := make(map[uuid.UUID]bool)

			log.Println("Devices online for user", toUserID, ":", devices)
			log.Println("Devices registered for user", toUserID, ":", userInfoNotification)

			for _, device := range userInfoNotification {
				deviceNeedToNotify[device.DeviceUuid] = true
			}

			for _, device := range devices {
				deviceUUID, err := uuid.Parse(device)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "failed to parse device UUID: %v", err)
				}

				delete(deviceNeedToNotify, deviceUUID)
			}

			pushTokens := make([]string, 0, len(deviceNeedToNotify))
			for _, device := range userInfoNotification {
				if _, ok := deviceNeedToNotify[device.DeviceUuid]; ok {
					pushTokens = append(pushTokens, device.PushToken)
				}
			}

			log.Println("Push tokens to notify:", pushTokens)

			msg := &messaging.MulticastMessage{
				Data: map[string]string{
					"conversation_id": req.ConversationId,
					"sender_id":       fmt.Sprintf("%d", userID),
					"title":           title,
					"avatar_url":      avatar_url,
					"message":         req.Message,
				},
				Fids: pushTokens,
			}

			resp, err := ns.messagingClient.SendEachForMulticast(ctx, msg)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to send message via FCM: %v", err)
			}

			log.Printf("Successfully sent message to %d devices, failed to send to %d devices", resp.SuccessCount, resp.FailureCount)
			for i, result := range resp.Responses {
				log.Printf(
					"FID[%d]: success=%v messageID=%s err=%v", i, result.Success, result.MessageID, result.Error)
			}
		}
	}

	return &notify_proto.SendToFIDResponse{
		Success: true,
	}, nil
}

type SendEachForMultiDevicesRequest struct {
	UserID         int32
	ConversationId string
	Message        string
}

func (ns *NotifyService) SendEachForMultiDevices(ctx context.Context, req SendEachForMultiDevicesRequest) (bool, error) {
	result, err := ns.chatClient.Client.GetConversationType(interceptor.WithUserIDMetadata(ctx, req.UserID), &chat_proto.GetConversationTypeRequest{
		ConversationId: req.ConversationId,
	})
	if err != nil {
		return false, status.Errorf(codes.Internal, "failed to get conversation type: %v", err)
	}

	var title, avatar_url string

	if result.ConversationType == ConversationTypeGroup {
		groupInfo, err := ns.chatClient.Client.GetGroupInfo(interceptor.WithUserIDMetadata(ctx, req.UserID), &chat_proto.GetGroupInfoRequest{
			ConversationId: req.ConversationId,
		})

		if err != nil {
			return false, status.Errorf(codes.Internal, "failed to get group info: %v", err)
		}

		title = groupInfo.GroupName
		avatar_url = groupInfo.GroupAvatarUrl

	} else {
		userInfo, err := ns.userClient.Client.GetProfile(interceptor.WithUserIDMetadata(ctx, req.UserID), &user_proto.GetProfileRequest{
			UserId: req.UserID,
		})

		if err != nil {
			return false, status.Errorf(codes.Internal, "failed to get user info: %v", err)
		}

		title = *userInfo.Name
		avatar_url = *userInfo.AvatarUrl

	}

	rows, err := ns.notify_repo.GetUserInfoByUserID(ctx, req.UserID)
	if err != nil {
		return false, status.Errorf(codes.Internal, "failed to get user info in notify data: %v", err)
	}

	pushTokens := make([]string, 0, len(rows))
	for _, row := range rows {
		pushTokens = append(pushTokens, row.PushToken)
	}

	msg := &messaging.MulticastMessage{
		Data: map[string]string{
			"conversation_id": req.ConversationId,
			"sender_id":       fmt.Sprintf("%d", req.UserID),
			"title":           title,
			"avatar_url":      avatar_url,
			"message":         req.Message,
		},
		Fids: pushTokens,
	}

	resp, err := ns.messagingClient.SendEachForMulticast(ctx, msg)
	if err != nil {
		return false, status.Errorf(codes.Internal, "failed to send message via FCM: %v", err)
	}
	log.Printf("Successfully sent message to %d devices, failed to send to %d devices", resp.SuccessCount, resp.FailureCount)

	return true, nil
}
