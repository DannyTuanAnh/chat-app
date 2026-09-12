package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/config"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db"
	chat_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/chat"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/interceptor"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/repository"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/service"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var chatPolicies = map[string][]string{}

type ChatServer struct {
	ctx    context.Context
	cfg    *config.Config
	server *grpc.Server
}

func NewChatServer(ctx context.Context, db db.ChatDB) (*ChatServer, error) {
	chatCertFile := utils.GetEnv("PATH_CERT_CHAT_SERVICE", "")
	chatKeyFile := utils.GetEnv("PATH_KEY_CHAT_SERVICE", "")

	var cert tls.Certificate
	var err error

	is_cloud_run := utils.GetEnv("IS_CLOUD_RUN", "false")
	if is_cloud_run == "true" {

		chatCertPEM := []byte(chatCertFile)
		chatKeyPEM := []byte(chatKeyFile)

		cert, err = tls.X509KeyPair(chatCertPEM, chatKeyPEM)
	} else {
		cert, err = tls.LoadX509KeyPair(chatCertFile, chatKeyFile)
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to load chat service TLS credentials: %v", err)
	}

	var caCert []byte

	if is_cloud_run == "true" {
		caCert = []byte(utils.GetEnv("PATH_CERT_CA", ""))
	} else {
		caCert, err = os.ReadFile(utils.GetEnv("PATH_CERT_CA", ""))
		if err != nil {
			return nil, fmt.Errorf("Failed to read CA cert: %v", err)
		}
	}

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},

		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  caPool,

		MinVersion: tls.VersionTLS12,
	}

	chatCfg := config.NewConfigChatService()

	cfg := &config.Config{}

	cfg.Service.ChatServiceAddr = chatCfg.Service.ChatServiceAddr
	cfg.Service.ChatServiceListenAddr = chatCfg.Service.ChatServiceListenAddr

	chat_repo := repository.NewChatRepository(db)
	chat_service := service.NewChatService(chat_repo)

	s := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.ChainUnaryInterceptor(
			interceptor.IdentityInterceptor(),
			interceptor.MTLSIdentityInterceptor(),
			interceptor.RBACInterceptor(chatPolicies),
			interceptor.ActiveUserInterceptor(chat_repo),
		),
	)

	chat_proto.RegisterChatServiceServer(s, chat_service)

	return &ChatServer{
		ctx:    ctx,
		cfg:    cfg,
		server: s,
	}, nil
}

func (fs *ChatServer) Run() (string, error) {
	listener, err := net.Listen("tcp", fs.cfg.Service.ChatServiceListenAddr)
	if err != nil {
		return "", fmt.Errorf("Failed to listen: %v", err)
	}

	errChan := make(chan error, 1)

	go func() {
		log.Printf("Chat server is listening on %s", listener.Addr())
		if err := fs.server.Serve(listener); err != nil {
			errChan <- fmt.Errorf("Failed to serve: %v", err)
		}
	}()

	select {
	case err := <-errChan:
		return "", fmt.Errorf("Chat server error: %v", err)
	case <-fs.ctx.Done():
		log.Println("Chat server is shutting down...")
		done := make(chan struct{})

		go func() {
			fs.server.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			return "Chat server stopped gracefully", nil
		case <-time.After(5 * time.Second):
			log.Println("Chat server shutdown timed out, forcing stop")
			fs.server.Stop()
		}
		return "Chat server stopped gracefully", nil
	}
}
