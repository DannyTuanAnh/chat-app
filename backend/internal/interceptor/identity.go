package interceptor

import (
	"context"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const UserIDContextKey contextKey = "user_id"

func IdentityInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(
				codes.Unauthenticated,
				"metadata not found",
			)
		}

		values := md.Get(string(UserIDContextKey))
		if len(values) == 0 {
			return nil, status.Error(
				codes.Unauthenticated,
				"user ID not found",
			)
		}

		id, err := strconv.ParseInt(values[0], 10, 32)
		if err != nil || id <= 0 {
			return nil, status.Error(
				codes.Unauthenticated,
				"invalid user ID",
			)
		}

		ctx = context.WithValue(
			ctx,
			UserIDContextKey,
			int32(id),
		)

		return handler(ctx, req)
	}
}

func WithUserIDMetadata(ctx context.Context, userID int32) context.Context {
	md := metadata.Pairs(string(UserIDContextKey), strconv.FormatInt(int64(userID), 10))

	return metadata.NewOutgoingContext(ctx, md)
}
