package client

import (
	notify_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/notify"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
)

type NotifyClient struct {
	Client notify_proto.NotifyServiceClient
}

func NewNotifyClient(addr string, certFile string, keyFile string) (*NotifyClient, error) {
	conn, err := NewGRPCConn(addr, utils.GetEnv("NOTIFY_SERVER_NAME", ""), certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &NotifyClient{
		Client: notify_proto.NewNotifyServiceClient(conn),
	}, nil
}
