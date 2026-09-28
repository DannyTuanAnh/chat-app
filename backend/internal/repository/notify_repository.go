package repository

import (
	"context"
	"errors"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/notify"
	"github.com/jackc/pgx/v5"
)

type notifyRepository struct {
	notify_repo db.NotifyDB
}

func NewNotifyRepository(db db.NotifyDB) NotifyRepository {
	return &notifyRepository{
		notify_repo: db,
	}
}

func (nr *notifyRepository) CreateUserInfo(ctx context.Context, arg sqlc.CreateFCMNotificationParams) (bool, error) {
	result, err := nr.notify_repo.DB.CreateFCMNotification(ctx, arg)
	if err != nil {
		return false, err
	}

	if result.RowsAffected() == 0 {
		return false, nil
	}

	return true, nil
}

func (nr *notifyRepository) GetUserInfo(ctx context.Context, arg sqlc.GetUserInfoParams) (sqlc.UserInfo, error) {
	row, err := nr.notify_repo.DB.GetUserInfo(ctx, arg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.UserInfo{}, ErrUserInfoNotFound
		}

		return sqlc.UserInfo{}, err
	}

	return row, nil
}

func (nr *notifyRepository) GetUserInfoByUserID(ctx context.Context, userID int32) ([]sqlc.GetUserInfoByUserIDRow, error) {
	rows, err := nr.notify_repo.DB.GetUserInfoByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserInfoNotFound
		}

		return nil, err
	}

	return rows, nil
}

func (nr *notifyRepository) IsUserDisabled(ctx context.Context, userID int32) (UserStatus, error) {
	row, err := nr.notify_repo.DB.GetDisabledUser(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserStatusActive, nil
		}

		return 0, err
	}

	return UserStatus(row.Status), nil
}
