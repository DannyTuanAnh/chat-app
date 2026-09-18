package repository

import (
	"context"
	"errors"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/chat"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
)

type chatRepository struct {
	chat_repository db.ChatDB
}

func NewChatRepository(db db.ChatDB) ChatRepository {
	return &chatRepository{
		chat_repository: db,
	}
}

func (cr *chatRepository) BeginTransaction(ctx context.Context) (pgx.Tx, error) {
	tx, err := cr.chat_repository.DBPool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	})
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (cr *chatRepository) RollBack(ctx context.Context, tx pgx.Tx, err *error) {
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

func (cr *chatRepository) IsUserDisabled(ctx context.Context, userID int32) (UserStatus, error) {
	row, err := cr.chat_repository.DB.GetDisabledUser(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserStatusActive, nil
		}

		return 0, err
	}

	return UserStatus(row.Status), nil
}

func (cr *chatRepository) CreateConversation(ctx context.Context, tx pgx.Tx, conversationType sqlc.ConversationType) (uuid.UUID, error) {
	if conversationType != sqlc.ConversationTypePrivate && conversationType != sqlc.ConversationTypeGroup {
		return uuid.Nil, errors.New("invalid conversation type")
	}

	if tx == nil {
		return uuid.Nil, errors.New("transaction is nil")
	}

	conversationID, err := cr.chat_repository.DB.WithTx(tx).CreateConversation(ctx, conversationType)
	if err != nil {
		return uuid.Nil, err
	}

	if conversationID == uuid.Nil {
		return uuid.Nil, errors.New("failed to create conversation")
	}

	return conversationID, nil
}

func (cr *chatRepository) AddMembersToConversation(ctx context.Context, tx pgx.Tx, conversationID uuid.UUID, userIDs []int32) error {
	if tx == nil {
		return errors.New("transaction is nil")
	}

	if len(userIDs) < 2 {
		return errors.New("at least two user IDs are required to add members to a conversation")
	}

	params := sqlc.AddMembersToConversationParams{
		ConversationID: conversationID,
		UserIds:        userIDs,
	}

	row, err := cr.chat_repository.DB.WithTx(tx).AddMembersToConversation(ctx, params)
	if err != nil {
		return err
	}

	if row.RowsAffected() == 0 || row.RowsAffected() != int64(len(userIDs)) {
		return errors.New("failed to add members to conversation")
	}

	return nil
}

func (cr *chatRepository) CreateSystemMessage(ctx context.Context, tx pgx.Tx, arg sqlc.CreateSystemMessageParams) (sqlc.SystemMessage, error) {
	if tx == nil {
		return sqlc.SystemMessage{}, errors.New("transaction is nil")
	}

	row, err := cr.chat_repository.DB.WithTx(tx).CreateSystemMessage(ctx, arg)
	if err != nil {
		return sqlc.SystemMessage{}, err
	}

	if row.ID == uuid.Nil {
		return sqlc.SystemMessage{}, errors.New("failed to create system message")
	}

	return row, nil
}
