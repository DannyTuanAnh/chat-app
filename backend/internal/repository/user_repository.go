package repository

import (
	"context"
	"errors"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/user"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type userRepository struct {
	user_repo db.UserDB
}

func NewUserRepository(db db.UserDB) UserRepository {
	return &userRepository{user_repo: db}
}

func (ur *userRepository) CreateUser(ctx context.Context, displayName string) (int32, error) {
	userID, err := ur.user_repo.DB.CreateUser(ctx, displayName)
	if err != nil {
		return 0, err
	}

	return userID, nil
}

func (ur *userRepository) DeleteUserByUserID(ctx context.Context, userId int32) error {
	result, err := ur.user_repo.DB.DeleteUser(ctx, userId)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrCannotDeleteUser
	}

	return nil
}

func (ur *userRepository) ActiveUser(ctx context.Context, userId int32) error {
	result, err := ur.user_repo.DB.ActiveUser(ctx, userId)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrCannotRestoreUser
	}

	return nil
}

func (ur *userRepository) IsExistProfile(ctx context.Context, userId int32) (bool, error) {
	exists, err := ur.user_repo.DB.IsExistProfile(ctx, userId)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (ur *userRepository) GetProfile(ctx context.Context, userId int32) (sqlc.GetProfileRow, error) {
	row, err := ur.user_repo.DB.GetProfile(ctx, userId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetProfileRow{}, ErrNotFoundProfile
		}

		return sqlc.GetProfileRow{}, err
	}

	return row, nil
}

func (ur *userRepository) GetProfileByUserID(ctx context.Context, arg sqlc.GetProfileByUserIdParams) (sqlc.GetProfileByUserIdRow, error) {
	row, err := ur.user_repo.DB.GetProfileByUserId(ctx, arg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetProfileByUserIdRow{}, ErrNotFoundProfileByUserID
		}

		return sqlc.GetProfileByUserIdRow{}, err
	}

	return row, nil
}

func (ur *userRepository) GetUserByUUID(ctx context.Context, targetUserUUID uuid.UUID) (sqlc.GetUserByUUIDRow, error) {
	row, err := ur.user_repo.DB.GetUserByUUID(ctx, targetUserUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetUserByUUIDRow{}, ErrNotFoundIdentityUUID
		}

		return sqlc.GetUserByUUIDRow{}, err
	}

	if !row.IsActive {
		return sqlc.GetUserByUUIDRow{}, ErrUserIsUnActive
	}

	return row, nil
}

func (ur *userRepository) CreateProfile(ctx context.Context, arg sqlc.CreateProfileParams) (sqlc.Profile, error) {
	return ur.user_repo.DB.CreateProfile(ctx, arg)
}

func (ur *userRepository) UpdateProfile(ctx context.Context, arg sqlc.UpdateProfileByUserIdParams) (sqlc.UpdateProfileByUserIdRow, error) {
	return ur.user_repo.DB.UpdateProfileByUserId(ctx, arg)
}

func (ur *userRepository) DisableUserByUserID(ctx context.Context, userId int32) error {
	return ur.user_repo.DB.DisableUser(ctx, userId)
}
