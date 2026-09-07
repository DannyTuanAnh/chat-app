package repository

import (
	"context"
	"errors"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/friend"
	"github.com/jackc/pgx/v5"
)

type friendRepository struct {
	friend_repo db.FriendDB
}

func NewFriendRepository(db db.FriendDB) FriendRepository {
	return &friendRepository{friend_repo: db}
}

func (fr *friendRepository) GetInfoRelationship(ctx context.Context, params sqlc.GetInfoRelationshipParams) (sqlc.GetInfoRelationshipRow, error) {
	info, err := fr.friend_repo.DB.GetInfoRelationship(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetInfoRelationshipRow{}, ErrSameUser
		}

		return sqlc.GetInfoRelationshipRow{}, err
	}

	return info, nil
}

func (fr *friendRepository) CreateFriendRequest(ctx context.Context, arg sqlc.AddFriendByIdParams) (sqlc.AddFriendByIdRow, error) {
	row, err := fr.friend_repo.DB.AddFriendById(ctx, arg)
	if err != nil {
		return sqlc.AddFriendByIdRow{}, err
	}

	return row, nil
}

func (fr *friendRepository) IsUserDisabled(ctx context.Context, userID int32) (UserStatus, error) {
	row, err := fr.friend_repo.DB.GetDisabledUser(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserStatusActive, nil
		}

		return 0, err
	}

	return UserStatus(row.Status), nil
}
