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

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/config"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db"
	friend_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/friend"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/interceptor"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/repository"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/service"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var friendPolicies = map[string][]string{
	"/proto.FriendService/GetRelationship": {
		os.Getenv("API_GATEWAY_NAME"),
	},
	"/proto.FriendService/SendFriendRequest": {
		os.Getenv("API_GATEWAY_NAME"),
	},
	"/proto.FriendService/RejectFriendRequest": {
		os.Getenv("API_GATEWAY_NAME"),
	},
	"/proto.FriendService/GetPendingFriendRequests": {
		os.Getenv("API_GATEWAY_NAME"),
	},
	"/proto.FriendService/GetSentFriendRequests": {
		os.Getenv("API_GATEWAY_NAME"),
	},
}

type FriendServer struct {
	ctx    context.Context
	cfg    *config.Config
	server *grpc.Server
}

func NewFriendServer(ctx context.Context, db db.FriendDB) (*FriendServer, error) {
	friendCertFile := utils.GetEnv("PATH_CERT_FRIEND_SERVICE", "")
	friendKeyFile := utils.GetEnv("PATH_KEY_FRIEND_SERVICE", "")

	friendCertFileClient := utils.GetEnv("PATH_CERT_FRIEND_SERVICE_CLIENT", "")
	friendKeyFileClient := utils.GetEnv("PATH_KEY_FRIEND_SERVICE_CLIENT", "")

	var cert tls.Certificate
	var err error

	is_cloud_run := utils.GetEnv("IS_CLOUD_RUN", "false")
	if is_cloud_run == "true" {

		friendCertPEM := []byte(friendCertFile)
		friendKeyPEM := []byte(friendKeyFile)

		cert, err = tls.X509KeyPair(friendCertPEM, friendKeyPEM)
	} else {
		cert, err = tls.LoadX509KeyPair(friendCertFile, friendKeyFile)
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to load friend service TLS credentials: %v", err)
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

	friendCfg := config.NewConfigFriendService()
	userCfg := config.NewConfigUserService()

	cfg := &config.Config{}

	cfg.Service.FriendServiceAddr = friendCfg.Service.FriendServiceAddr
	cfg.Service.UserServiceAddr = userCfg.Service.UserServiceAddr

	cfg.Service.FriendServiceListenAddr = friendCfg.Service.FriendServiceListenAddr

	user_client, err := client.NewUserClient(cfg.Service.UserServiceAddr, friendCertFileClient, friendKeyFileClient)
	if err != nil {
		return nil, fmt.Errorf("Failed to create user client: %v", err)
	}

	friend_repo := repository.NewFriendRepository(db)
	friend_service := service.NewFriendService(friend_repo, user_client)

	s := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.ChainUnaryInterceptor(
			interceptor.IdentityInterceptor(),
			interceptor.MTLSIdentityInterceptor(),
			interceptor.RBACInterceptor(friendPolicies),
			interceptor.ActiveUserInterceptor(friend_repo),
		),
	)

	friend_proto.RegisterFriendServiceServer(s, friend_service)

	return &FriendServer{
		ctx:    ctx,
		cfg:    cfg,
		server: s,
	}, nil
}

func (fs *FriendServer) Run() (string, error) {
	listener, err := net.Listen("tcp", fs.cfg.Service.FriendServiceListenAddr)
	if err != nil {
		return "", fmt.Errorf("Failed to listen: %v", err)
	}

	errChan := make(chan error, 1)

	go func() {
		log.Printf("Friend server is listening on %s", listener.Addr())
		if err := fs.server.Serve(listener); err != nil {
			errChan <- fmt.Errorf("Failed to serve: %v", err)
		}
	}()

	select {
	case err := <-errChan:
		return "", fmt.Errorf("Friend server error: %v", err)
	case <-fs.ctx.Done():
		log.Println("Friend server is shutting down...")
		done := make(chan struct{})

		go func() {
			fs.server.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			return "Friend server stopped gracefully", nil
		case <-time.After(5 * time.Second):
			log.Println("Friend server shutdown timed out, forcing stop")
			fs.server.Stop()
		}
		return "Friend server stopped gracefully", nil
	}
}
