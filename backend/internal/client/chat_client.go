package client

import (
	chat_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/chat"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
)

type ChatClient struct {
	Client chat_proto.ChatServiceClient
}

func NewChatClient(addr string, certFile string, keyFile string) (*ChatClient, error) {
	conn, err := NewGRPCConn(addr, utils.GetEnv("CHAT_SERVER_NAME", ""), certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &ChatClient{
		Client: chat_proto.NewChatServiceClient(conn),
	}, nil
}
