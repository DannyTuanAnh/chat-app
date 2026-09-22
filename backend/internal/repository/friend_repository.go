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

func (fr *friendRepository) BeginTransaction(ctx context.Context) (pgx.Tx, error) {
	tx, err := fr.friend_repo.DBPool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})

	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (fr *friendRepository) RollBack(ctx context.Context, tx pgx.Tx, err *error) {
	if tx == nil {
		return
	}

	if p := recover(); p != nil {
		tx.Rollback(ctx)
		panic(p)
	}

	if err != nil && *err != nil {
		tx.Rollback(ctx)
	}
}

func (fr *friendRepository) GetFriendList(ctx context.Context, arg sqlc.GetFriendListParams) ([]int32, error) {
	rows, err := fr.friend_repo.DB.GetFriendList(ctx, arg)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (fr *friendRepository) AcceptFriendRequestById(ctx context.Context, tx pgx.Tx, arg sqlc.AcceptFriendRequestByIdParams) (sqlc.AcceptFriendRequestByIdRow, error) {
	if tx == nil {
		return sqlc.AcceptFriendRequestByIdRow{}, errors.New("transaction is nil")
	}

	row, err := fr.friend_repo.DB.WithTx(tx).AcceptFriendRequestById(ctx, arg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.AcceptFriendRequestByIdRow{}, ErrNoRowsAcceptFriendRequestAffected
		}
		return sqlc.AcceptFriendRequestByIdRow{}, err
	}

	if !row.IsAccepted {
		return sqlc.AcceptFriendRequestByIdRow{}, ErrNoRowsAcceptFriendRequestAffected
	}

	return row, nil
}

func (fr *friendRepository) CreateFriendShip(ctx context.Context, tx pgx.Tx, arg sqlc.CreateFriendShipParams) error {
	if tx == nil {
		return errors.New("transaction is nil")
	}

	result, err := fr.friend_repo.DB.WithTx(tx).CreateFriendShip(ctx, arg)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrFriendshipAlreadyLatest
	}

	return nil
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

func (fr *friendRepository) RejectFriendRequest(ctx context.Context, arg sqlc.RejectFriendRequestByIdParams) error {
	row, err := fr.friend_repo.DB.RejectFriendRequestById(ctx, arg)
	if err != nil {
		return err
	}

	if row.RowsAffected() == 0 {
		return ErrNoRowsRejectFriendRequestAffected
	}

	return nil
}

func (fr *friendRepository) GetPendingFriendRequests(ctx context.Context, arg sqlc.GetPendingFriendRequestsParams) ([]sqlc.GetPendingFriendRequestsRow, error) {
	rows, err := fr.friend_repo.DB.GetPendingFriendRequests(ctx, arg)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (fr *friendRepository) GetSentFriendRequests(ctx context.Context, arg sqlc.GetSentFriendRequestsParams) ([]sqlc.GetSentFriendRequestsRow, error) {
	rows, err := fr.friend_repo.DB.GetSentFriendRequests(ctx, arg)
	if err != nil {
		return nil, err
	}

	return rows, nil
}
