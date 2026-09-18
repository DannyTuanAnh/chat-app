package repository

import (
	"context"

	sqlc_auth "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/auth"
	sqlc_chat "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/chat"
	sqlc_friend "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/friend"
	sqlc_user "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/user"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserStatus int16

const (
	UserStatusDisabled UserStatus = 1
	UserStatusDeleted  UserStatus = 2
	UserStatusActive   UserStatus = 3
)

type APIKeyRepository interface {
	CreateAPIKey(ctx context.Context, keyHash string) error
	RevokeAPIKey(ctx context.Context, keyHash string) error
	RevokeAll(ctx context.Context) error
}

type AuthRepository interface {
	IsExistingIdentityID(ctx context.Context, arg sqlc_auth.FindExistingIdentityParams) (sqlc_auth.FindExistingIdentityRow, error)
	CreateIdentity(ctx context.Context, arg sqlc_auth.CreateIdentityParams) error
	ActiveIdentity(ctx context.Context, arg sqlc_auth.ActiveIdentityParams) error
	DisableIdentity(ctx context.Context, arg sqlc_auth.DisableIdentityParams) error
	CreateSession(ctx context.Context, userID int32) (uuid.UUID, error)
	CheckSession(ctx context.Context, sessionID uuid.UUID) (sqlc_auth.CheckSessionRow, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	LogoutAll(ctx context.Context, userId int32) error
}

type UserRepository interface {
	CreateUser(ctx context.Context, displayName string) (int32, error)
	DeleteUserByUserID(ctx context.Context, userId int32) error
	ActiveUser(ctx context.Context, userId int32) error
	IsExistProfile(ctx context.Context, userId int32) (bool, error)
	GetProfile(ctx context.Context, userId int32) (sqlc_user.GetProfileRow, error)
	GetProfileByUserID(ctx context.Context, arg sqlc_user.GetProfileByUserIdParams) (sqlc_user.GetProfileByUserIdRow, error)
	GetProfileByUserIDs(ctx context.Context, userIDs []int32) ([]sqlc_user.GetProfileByUserIDsRow, error)
	GetUserByUUID(ctx context.Context, targetUserUUID uuid.UUID) (sqlc_user.GetUserByUUIDRow, error)
	CreateProfile(ctx context.Context, arg sqlc_user.CreateProfileParams) (sqlc_user.Profile, error)
	DisableUserByUserID(ctx context.Context, userId int32) error
	UpdateProfile(ctx context.Context, arg sqlc_user.UpdateProfileByUserIdParams) (sqlc_user.UpdateProfileByUserIdRow, error)
}

type FriendRepository interface {
	BeginTransaction(ctx context.Context) (pgx.Tx, error)
	RollBack(ctx context.Context, tx pgx.Tx, err *error)

	GetInfoRelationship(ctx context.Context, params sqlc_friend.GetInfoRelationshipParams) (sqlc_friend.GetInfoRelationshipRow, error)

	CreateFriendRequest(ctx context.Context, arg sqlc_friend.AddFriendByIdParams) (sqlc_friend.AddFriendByIdRow, error)
	GetPendingFriendRequests(ctx context.Context, arg sqlc_friend.GetPendingFriendRequestsParams) ([]sqlc_friend.GetPendingFriendRequestsRow, error)
	GetSentFriendRequests(ctx context.Context, arg sqlc_friend.GetSentFriendRequestsParams) ([]sqlc_friend.GetSentFriendRequestsRow, error)
	RejectFriendRequest(ctx context.Context, arg sqlc_friend.RejectFriendRequestByIdParams) error

	AcceptFriendRequestById(ctx context.Context, tx pgx.Tx, arg sqlc_friend.AcceptFriendRequestByIdParams) (sqlc_friend.AcceptFriendRequestByIdRow, error)
	CreateFriendShip(ctx context.Context, tx pgx.Tx, arg sqlc_friend.CreateFriendShipParams) error

	IsUserDisabled(ctx context.Context, userID int32) (UserStatus, error)
}

type ChatRepository interface {
	BeginTransaction(ctx context.Context) (pgx.Tx, error)
	RollBack(ctx context.Context, tx pgx.Tx, err *error)

	IsUserDisabled(ctx context.Context, userID int32) (UserStatus, error)

	CreateConversation(ctx context.Context, tx pgx.Tx, conversationType sqlc_chat.ConversationType) (uuid.UUID, error)
	AddMembersToConversation(ctx context.Context, tx pgx.Tx, conversationID uuid.UUID, userIDs []int32) error
	CreateSystemMessage(ctx context.Context, tx pgx.Tx, arg sqlc_chat.CreateSystemMessageParams) (sqlc_chat.SystemMessage, error)
}

type NotifyRepository interface {
}
