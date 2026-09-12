package app

import (
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/handler"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/routes"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
)

type ChatModule struct {
	routes routes.Routes
}

func NewChatModule(addr string) *ChatModule {
	// Load TLS credentials for gRPC client
	// Call by API Gateway, so use API Gateway's certs
	apiGatewayCertFile := utils.GetEnv("PATH_CERT_API_GATEWAY_CLIENT", "")
	apiGatewayKeyFile := utils.GetEnv("PATH_KEY_API_GATEWAY_CLIENT", "")

	// 1. Initialize chat client
	chat_client, err := client.NewChatClient(addr, apiGatewayCertFile, apiGatewayKeyFile)
	if err != nil {
		panic("Failed to initialize Chat client: " + err.Error())
	}

	// 3. Initialize handler
	chat_handler := handler.NewChatHandler(chat_client)

	// 4. Initialize routes
	chat_routes := routes.NewChatRoutes(chat_handler)

	return &ChatModule{routes: chat_routes}
}

func (us *ChatModule) Routes() routes.Routes {
	return us.routes
}
