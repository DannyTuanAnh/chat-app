package interceptor

import (
	"context"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ActiveUserInterceptor(repo any) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		validRepo, ok := repo.(interface {
			IsUserDisabled(ctx context.Context, userID int32) (repository.UserStatus, error)
		})
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "invalid repository type")
		}

		userID, ok := ctx.Value(UserIDContextKey).(int32)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "user ID not found in context")
		}

		userStatus, err := validRepo.IsUserDisabled(ctx, userID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to check user status: %v", err)
		}

		switch userStatus {
		case repository.UserStatusDisabled:
			return nil, status.Error(codes.PermissionDenied, "user is disabled")
		case repository.UserStatusDeleted:
			return nil, status.Error(codes.PermissionDenied, "user is deleted")
		case repository.UserStatusActive:
			// User is active, proceed with the request
		default:
			return nil, status.Error(codes.Internal, "unknown user status")
		}

		return handler(ctx, req)
	}
}
