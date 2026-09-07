package app

import (
	"context"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/handler"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/routes"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
)

type FriendModule struct {
	routes routes.Routes
}

func NewFriendModule(addr string, ctx context.Context) *FriendModule {
	// Load TLS credentials for gRPC client
	// Call by API Gateway, so use API Gateway's certs
	apiGatewayCertFile := utils.GetEnv("PATH_CERT_API_GATEWAY_CLIENT", "")
	apiGatewayKeyFile := utils.GetEnv("PATH_KEY_API_GATEWAY_CLIENT", "")

	// 1. Initialize friend client
	friend_client, err := client.NewFriendClient(addr, apiGatewayCertFile, apiGatewayKeyFile)
	if err != nil {
		panic("Failed to initialize Friend client: " + err.Error())
	}

	// 3. Initialize handler
	friend_handler := handler.NewFriendHandler(friend_client)

	// 4. Initialize routes
	friend_routes := routes.NewFriendRoutes(friend_handler)

	return &FriendModule{routes: friend_routes}
}

func (us *FriendModule) Routes() routes.Routes {
	return us.routes
}
