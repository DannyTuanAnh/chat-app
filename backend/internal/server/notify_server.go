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

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/config"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db"
	notify_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/notify"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/interceptor"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/repository"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/service"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/redis/go-redis/v9"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var notifyPolicies = map[string][]string{
	"/proto.NotifyService/RegisterDevice": {
		os.Getenv("API_GATEWAY_NAME"),
	},

	"/proto.NotifyService/SendToFID": {
		os.Getenv("CHAT_SERVICE_NAME"),
	},
}

type NotifyServer struct {
	ctx    context.Context
	cfg    *config.Config
	server *grpc.Server
}

func NewNotifyServer(ctx context.Context, db db.NotifyDB, rdb *redis.Client) (*NotifyServer, error) {
	notifyCertFile := utils.GetEnv("PATH_CERT_NOTIFY_SERVICE", "")
	notifyKeyFile := utils.GetEnv("PATH_KEY_NOTIFY_SERVICE", "")

	notifyCertFileClient := utils.GetEnv("PATH_CERT_NOTIFY_SERVICE_CLIENT", "")
	notifyKeyFileClient := utils.GetEnv("PATH_KEY_NOTIFY_SERVICE_CLIENT", "")

	var cert tls.Certificate
	var err error

	is_cloud_run := utils.GetEnv("IS_CLOUD_RUN", "false")
	if is_cloud_run == "true" {

		notifyCertPEM := []byte(notifyCertFile)
		notifyKeyPEM := []byte(notifyKeyFile)

		cert, err = tls.X509KeyPair(notifyCertPEM, notifyKeyPEM)
	} else {
		cert, err = tls.LoadX509KeyPair(notifyCertFile, notifyKeyFile)
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to load notify service TLS credentials: %v", err)
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

	notifyCfg := config.NewConfigNotifyService()
	userCfg := config.NewConfigUserService()
	chatCfg := config.NewConfigChatService()

	cfg := &config.Config{}

	cfg.Service.NotifyServiceAddr = notifyCfg.Service.NotifyServiceAddr
	cfg.Service.UserServiceAddr = userCfg.Service.UserServiceAddr
	cfg.Service.ChatServiceAddr = chatCfg.Service.ChatServiceAddr

	cfg.Service.NotifyServiceListenAddr = notifyCfg.Service.NotifyServiceListenAddr

	user_client, err := client.NewUserClient(cfg.Service.UserServiceAddr, notifyCertFileClient, notifyKeyFileClient)
	if err != nil {
		return nil, fmt.Errorf("Failed to create user client: %v", err)
	}

	chat_client, err := client.NewChatClient(cfg.Service.ChatServiceAddr, notifyCertFileClient, notifyKeyFileClient)
	if err != nil {
		return nil, fmt.Errorf("Failed to create chat client: %v", err)
	}

	firebaseMessagingClient := connectMessagingFirebase(ctx)

	notify_repo := repository.NewNotifyRepository(db)
	notify_service := service.NewNotifyService(notify_repo, user_client, chat_client, firebaseMessagingClient, rdb)

	s := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.ChainUnaryInterceptor(
			interceptor.IdentityInterceptor(),
			interceptor.MTLSIdentityInterceptor(),
			interceptor.RBACInterceptor(notifyPolicies),
			interceptor.ActiveUserInterceptor(notify_repo),
		),
	)

	notify_proto.RegisterNotifyServiceServer(s, notify_service)

	return &NotifyServer{
		ctx:    ctx,
		cfg:    cfg,
		server: s,
	}, nil
}

func (fs *NotifyServer) Run() (string, error) {
	listener, err := net.Listen("tcp", fs.cfg.Service.NotifyServiceListenAddr)
	if err != nil {
		return "", fmt.Errorf("Failed to listen: %v", err)
	}

	errChan := make(chan error, 1)

	go func() {
		log.Printf("Notify server is listening on %s", listener.Addr())
		if err := fs.server.Serve(listener); err != nil {
			errChan <- fmt.Errorf("Failed to serve: %v", err)
		}
	}()

	select {
	case err := <-errChan:
		return "", fmt.Errorf("Notify server error: %v", err)
	case <-fs.ctx.Done():
		log.Println("Notify server is shutting down...")
		done := make(chan struct{})

		go func() {
			fs.server.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			return "Notify server stopped gracefully", nil
		case <-time.After(5 * time.Second):
			log.Println("Notify server shutdown timed out, forcing stop")
			fs.server.Stop()
		}
		return "Notify server stopped gracefully", nil
	}
}

func connectMessagingFirebase(ctx context.Context) *messaging.Client {
	// Initialize Firebase app for verify otp
	serviceAccountKey := utils.GetEnv("FIREBASE_CLOUD_MESSAGING_CREDENTIALS", "")

	var app *firebase.App
	var err error
	if serviceAccountKey != "" {
		var opt option.ClientOption

		//local test
		opt = option.WithAuthCredentialsFile(option.ServiceAccount, serviceAccountKey)

		// // deploy
		// if strings.HasPrefix(serviceAccountKey, "/") {
		// 	opt = option.WithAuthCredentialsFile(option.ServiceAccount, serviceAccountKey)
		// } else {
		// 	opt = option.WithAuthCredentialsJSON(option.ServiceAccount, []byte(serviceAccountKey))
		// }

		app, err = firebase.NewApp(ctx, nil, opt)

	} else {
		app, err = firebase.NewApp(ctx, nil)
	}

	if err != nil {
		panic("Failed to initialize Firebase app: " + err.Error())
	}

	messagingClient, err := app.Messaging(ctx)
	if err != nil {
		panic("Failed to initialize Firebase Auth client: " + err.Error())
	}

	return messagingClient
}
