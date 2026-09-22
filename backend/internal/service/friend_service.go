package service

import (
	"context"
	"fmt"
	"time"

	"buf.build/go/protovalidate"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/friend"
	chat_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/chat"
	friend_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/friend"
	user_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/user"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/interceptor"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/repository"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type friendService struct {
	friend_proto.UnimplementedFriendServiceServer
	friend_repo repository.FriendRepository
	user_client *client.UserClient
	chat_client *client.ChatClient
	validator   protovalidate.Validator
}

func NewFriendService(friend_repo repository.FriendRepository, user_client *client.UserClient, chat_client *client.ChatClient) *friendService {
	v, err := protovalidate.New()
	if err != nil {
		panic(fmt.Sprintf("Failed to create validator: %v", err))
	}

	return &friendService{
		friend_repo: friend_repo,
		user_client: user_client,
		chat_client: chat_client,
		validator:   v,
	}
}

func (fs *friendService) GetFriendList(ctx context.Context, req *friend_proto.GetFriendListRequest) (*friend_proto.GetFriendListResponse, error) {
	if err := fs.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	arg := sqlc.GetFriendListParams{
		CurrentUserID: req.CurrentUserId,
		LastFriendID:  req.LastFriendId,
	}

	friendIDs, err := fs.friend_repo.GetFriendList(ctx, arg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get friend list: %v", err)
	}

	if len(friendIDs) == 0 {
		return &friend_proto.GetFriendListResponse{
			FriendList: []*friend_proto.UserSummary{},
		}, nil
	}

	request := &user_proto.GetProfileByUserIDsRequest{
		UserIds: friendIDs,
	}

	userData, err := fs.user_client.Client.GetProfileByUserIDs(interceptor.WithUserIDMetadata(ctx, req.CurrentUserId), request)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get pending friend requests: %v", err)
	}

	userByID := make(map[int32]*user_proto.GetProfileByUserIDsResponse_UserProfile, len(userData.Profiles))
	for _, profile := range userData.Profiles {
		userByID[profile.UserId] = profile
	}

	friendList := make([]*friend_proto.UserSummary, 0, len(friendIDs))
	for _, friendID := range friendIDs {
		userProfile, ok := userByID[friendID]
		if !ok {
			return nil, status.Errorf(codes.Internal, "Failed to get friend list: user profile not found for friend ID %d", friendID)
		}

		friend := &friend_proto.UserSummary{
			UserId:    userProfile.UserId,
			Username:  userProfile.Name,
			AvatarUrl: userProfile.AvatarUrl,
		}

		friendList = append(friendList, friend)
	}

	return &friend_proto.GetFriendListResponse{
		FriendList: friendList,
	}, nil
}

func (fs *friendService) GetRelationship(ctx context.Context, req *friend_proto.GetRelationshipRequest) (*friend_proto.GetRelationshipResponse, error) {
	if err := fs.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	info, err := fs.friend_repo.GetInfoRelationship(ctx, sqlc.GetInfoRelationshipParams{
		CurrentUserID: req.CurrentUserId,
		TargetUserID:  req.TargetUserId,
	})
	if err != nil {
		if err == repository.ErrSameUser {
			return nil, status.Errorf(codes.InvalidArgument, "Cannot perform action on the same user")
		}

		return nil, status.Errorf(codes.Internal, "Failed to get relationship info: %v", err)
	}

	if info.IsFriend {
		return &friend_proto.GetRelationshipResponse{
			FriendRequestDirection: nil,
			IsFriend:               true,
		}, nil

	}

	if info.SenderID == nil && info.IsAccepted == nil {
		return &friend_proto.GetRelationshipResponse{
			FriendRequestDirection: nil,
			IsFriend:               false,
		}, nil
	}

	if !*info.IsAccepted {
		if *info.SenderID == req.CurrentUserId {
			direction := utils.StringPtr("sent")

			return &friend_proto.GetRelationshipResponse{
				FriendRequestDirection: direction,
				IsFriend:               false,
			}, nil
		}

		direction := utils.StringPtr("received")

		return &friend_proto.GetRelationshipResponse{
			FriendRequestDirection: direction,
			IsFriend:               false,
		}, nil

	}

	return nil, status.Errorf(codes.Internal, "Something went wrong while determining the relationship status, may be error in the table data relating to the relationship between users %d and %d, please try again later or contact support", req.CurrentUserId, req.TargetUserId)
}

func (fs *friendService) SendFriendRequest(ctx context.Context, req *friend_proto.SendFriendRequestRequest) (*friend_proto.SendFriendRequestResponse, error) {
	if err := fs.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	result, err := fs.friend_repo.CreateFriendRequest(ctx, sqlc.AddFriendByIdParams{
		SenderUserID:   req.CurrentUserId,
		ReceiverUserID: req.TargetUserId,
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to send friend request: %v", err)
	}

	if result.FStatus != true {
		return nil, status.Errorf(codes.Internal, "Failed to send friend request: %s", result.FMessage)
	}

	return &friend_proto.SendFriendRequestResponse{
		Success: true,
	}, nil
}

func (fs *friendService) RejectFriendRequest(ctx context.Context, req *friend_proto.RejectFriendRequestRequest) (*friend_proto.RejectFriendRequestResponse, error) {
	if err := fs.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	err := fs.friend_repo.RejectFriendRequest(ctx, sqlc.RejectFriendRequestByIdParams{
		RequestID:  req.RequestId,
		ReceiverID: req.CurrentUserId,
	})

	if err != nil {
		if err == repository.ErrNoRowsRejectFriendRequestAffected {
			return nil, status.Errorf(codes.NotFound, "Failed to reject friend request: %v", err)
		}

		return nil, status.Errorf(codes.Internal, "Failed to reject friend request: %v", err)
	}

	return &friend_proto.RejectFriendRequestResponse{
		Success: true,
	}, nil
}

func (fs *friendService) GetPendingFriendRequests(ctx context.Context, req *friend_proto.GetPendingFriendRequestsRequest) (*friend_proto.GetPendingFriendRequestsResponse, error) {
	if err := fs.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	rows, err := fs.friend_repo.GetPendingFriendRequests(ctx, sqlc.GetPendingFriendRequestsParams{
		CurrentUserID: req.CurrentUserId,
		LastRequestID: req.LastRequestId,
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get pending friend requests: %v", err)
	}

	if len(rows) == 0 {
		return &friend_proto.GetPendingFriendRequestsResponse{
			PendingRequests: []*friend_proto.PendingFriendRequest{},
		}, nil
	}

	senderIDs := make([]int32, 0, len(rows))
	for _, row := range rows {
		senderIDs = append(senderIDs, row.SenderID)
	}

	request := &user_proto.GetProfileByUserIDsRequest{
		UserIds: senderIDs,
	}

	userData, err := fs.user_client.Client.GetProfileByUserIDs(interceptor.WithUserIDMetadata(ctx, req.CurrentUserId), request)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get pending friend requests: %v", err)
	}

	userByID := make(map[int32]*user_proto.GetProfileByUserIDsResponse_UserProfile, len(userData.Profiles))
	for _, profile := range userData.Profiles {
		userByID[profile.UserId] = profile
	}

	pendingRequests := make([]*friend_proto.PendingFriendRequest, 0, len(rows))
	for _, row := range rows {
		userProfile, ok := userByID[row.SenderID]
		if !ok {
			return nil, status.Errorf(codes.Internal, "Failed to get pending friend requests: user profile not found for sender ID %d", row.SenderID)
		}

		pendingRequests = append(pendingRequests, &friend_proto.PendingFriendRequest{
			RequestId: row.RequestID,
			Sender: &friend_proto.UserSummary{
				UserId:    userProfile.UserId,
				Username:  userProfile.Name,
				AvatarUrl: userProfile.AvatarUrl,
			},
			SendAt: row.SendAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &friend_proto.GetPendingFriendRequestsResponse{
		PendingRequests: pendingRequests,
	}, nil
}

func (fs *friendService) GetSentFriendRequests(ctx context.Context, req *friend_proto.GetSentFriendRequestsRequest) (*friend_proto.GetSentFriendRequestsResponse, error) {
	if err := fs.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	rows, err := fs.friend_repo.GetSentFriendRequests(ctx, sqlc.GetSentFriendRequestsParams{
		CurrentUserID: req.CurrentUserId,
		LastRequestID: req.LastRequestId,
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get sent friend requests: %v", err)
	}

	if len(rows) == 0 {
		return &friend_proto.GetSentFriendRequestsResponse{
			SentRequests: []*friend_proto.SentFriendRequest{},
		}, nil
	}

	receiverIDs := make([]int32, 0, len(rows))
	for _, row := range rows {
		receiverIDs = append(receiverIDs, row.ReceiverID)
	}

	request := &user_proto.GetProfileByUserIDsRequest{
		UserIds: receiverIDs,
	}

	userData, err := fs.user_client.Client.GetProfileByUserIDs(interceptor.WithUserIDMetadata(ctx, req.CurrentUserId), request)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get sent friend requests: %v", err)
	}

	userByID := make(map[int32]*user_proto.GetProfileByUserIDsResponse_UserProfile, len(userData.Profiles))
	for _, profile := range userData.Profiles {
		userByID[profile.UserId] = profile
	}

	sentRequests := make([]*friend_proto.SentFriendRequest, 0, len(rows))
	for _, row := range rows {
		userProfile, ok := userByID[row.ReceiverID]
		if !ok {
			return nil, status.Errorf(codes.Internal, "Failed to get sent friend requests: user profile not found for receiver ID %d", row.ReceiverID)
		}

		sentRequests = append(sentRequests, &friend_proto.SentFriendRequest{
			RequestId: row.RequestID,
			Receiver: &friend_proto.UserSummary{
				UserId:    userProfile.UserId,
				Username:  userProfile.Name,
				AvatarUrl: userProfile.AvatarUrl,
			},
			SendAt: row.SendAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &friend_proto.GetSentFriendRequestsResponse{
		SentRequests: sentRequests,
	}, nil
}

func (fs *friendService) AcceptFriendRequest(ctx context.Context, req *friend_proto.AcceptFriendRequestRequest) (*friend_proto.AcceptFriendRequestResponse, error) {
	if err := fs.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	currentUserID, ok := ctx.Value(interceptor.UserIDContextKey).(int32)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user ID not found in context")
	}

	tx, err := fs.friend_repo.BeginTransaction(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to begin transaction: %v", err)
	}
	defer fs.friend_repo.RollBack(ctx, tx, &err)

	acceptResult, err := fs.friend_repo.AcceptFriendRequestById(ctx, tx, sqlc.AcceptFriendRequestByIdParams{
		RequestID:  req.RequestId,
		ReceiverID: currentUserID,
	})

	if err != nil {
		if err == repository.ErrNoRowsAcceptFriendRequestAffected {
			return nil, status.Errorf(codes.NotFound, "Failed to accept friend request: %v", err)
		}

		return nil, status.Errorf(codes.Internal, "Failed to accept friend request: %v", err)
	}

	err = fs.friend_repo.CreateFriendShip(ctx, tx, sqlc.CreateFriendShipParams{
		SenderUserID:   acceptResult.SenderID,
		ReceiverUserID: currentUserID,
		EstablishedAt:  time.Now(),
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create friendship: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to commit transaction: %v", err)
	}

	_, err = fs.chat_client.Client.CreatePrivateConversation(interceptor.WithUserIDMetadata(ctx, currentUserID), &chat_proto.CreatePrivateConversationRequest{
		UserId_1: acceptResult.SenderID,
		UserId_2: acceptResult.ReceiverID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create private conversation: %v", err)
	}

	return &friend_proto.AcceptFriendRequestResponse{
		Success: true,
	}, nil
}
