package repository

import "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db"

type chatRepository struct {
	chat_repository db.ChatDB
}

func NewChatRepository(db db.ChatDB) ChatRepository {
	return &chatRepository{
		chat_repository: db,
	}
}
