package service

import (
	"context"
	"fmt"

	"buf.build/go/protovalidate"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/friend"
	friend_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/friend"
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
	validator   protovalidate.Validator
}

func NewFriendService(friend_repo repository.FriendRepository, user_client *client.UserClient) *friendService {
	v, err := protovalidate.New()
	if err != nil {
		panic(fmt.Sprintf("Failed to create validator: %v", err))
	}

	return &friendService{
		friend_repo: friend_repo,
		user_client: user_client,
		validator:   v,
	}
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
