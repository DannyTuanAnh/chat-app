package repository

import (
	"context"
	"errors"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type authRepository struct {
	auth_repo db.AuthDB
}

func NewAuthRepository(db db.AuthDB) AuthRepository {
	return &authRepository{auth_repo: db}
}

func (ar *authRepository) IsExistingIdentityID(ctx context.Context, arg sqlc.FindExistingIdentityParams) (sqlc.FindExistingIdentityRow, error) {
	row, err := ar.auth_repo.DB.FindExistingIdentity(ctx, arg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.FindExistingIdentityRow{}, ErrNotFoundIdentityID
		}

		return sqlc.FindExistingIdentityRow{}, err
	}

	return row, nil
}

func (ar *authRepository) CreateIdentity(ctx context.Context, arg sqlc.CreateIdentityParams) error {
	err := ar.auth_repo.DB.CreateIdentity(ctx, arg)
	if err != nil {
		var pqErr *pgconn.PgError
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return ErrIdentityAlreadyExist
		}

		return err
	}

	return nil
}

func (ar *authRepository) ActiveIdentity(ctx context.Context, arg sqlc.ActiveIdentityParams) error {
	result, err := ar.auth_repo.DB.ActiveIdentity(ctx, arg)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrCannotRestoreUserIdentity
	}

	return nil
}

func (ar *authRepository) DisableIdentity(ctx context.Context, arg sqlc.DisableIdentityParams) error {
	result, err := ar.auth_repo.DB.DisableIdentity(ctx, arg)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFoundIdentityID
	}

	return nil
}

func (ar *authRepository) CreateDevice(ctx context.Context, arg sqlc.CreateDeviceParams) error {
	result, err := ar.auth_repo.DB.CreateDevice(ctx, arg)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrCannotCreateDevice
	}

	return nil
}

func (ar *authRepository) CreateSession(ctx context.Context, arg sqlc.CreateSessionParams) (uuid.UUID, error) {
	sessionID, err := ar.auth_repo.DB.CreateSession(ctx, arg)
	if err != nil {
		return uuid.Nil, err
	}

	return sessionID, nil
}

func (ar *authRepository) Logout(ctx context.Context, arg sqlc.RevokeSessionAndDeviceParams) error {
	err := ar.auth_repo.DB.RevokeSessionAndDevice(ctx, arg)
	if err != nil {
		return err
	}

	return nil
}

func (ar *authRepository) LogoutAll(ctx context.Context, userId int32) error {
	err := ar.auth_repo.DB.RevokeAllSessions(ctx, userId)
	if err != nil {
		return err
	}

	return nil
}
