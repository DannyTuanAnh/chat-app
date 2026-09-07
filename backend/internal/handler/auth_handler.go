package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/dto"
	auth_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/auth"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/middleware"

	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth_client *client.AuthClient
}

func NewAuthHandler(auth_client *client.AuthClient) *AuthHandler {
	return &AuthHandler{
		auth_client: auth_client,
	}
}

func (h *AuthHandler) LoginGoogle(ctx *gin.Context) {
	var input dto.RequestLoginGoogle

	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	authReq := &auth_proto.LoginRequest{
		AuthorCode: input.AuthCode,
	}

	resp, err := h.auth_client.Client.LoginGoogle(ctx, authReq)
	if err != nil {
		utils.WriteGRPCErrorToGin(ctx, fmt.Errorf("failed to login with Google: %v", err))
		return
	}

	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     "session_id",
		Value:    resp.Session,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Domain:   utils.GetEnv("COOKIE_DOMAIN", ""),
		Path:     "/",
		MaxAge:   utils.GetEnvInt("SESSION_ID_MAX_AGE", 168) * 3600,
	})

	utils.ResponseSuccess(ctx, http.StatusOK)
}

func (h *AuthHandler) Logout(ctx *gin.Context) {
	sessionID, exist := ctx.Get("session_id")
	if !exist {
		utils.ResponseErrorAbort(ctx, utils.NewError("session_id not found in context", utils.ErrCodeUnauthorized))
		return
	}

	req := &auth_proto.LogoutRequest{
		SessionId: sessionID.(string),
	}

	_, err := h.auth_client.Client.Logout(ctx, req)
	if err != nil {
		utils.WriteGRPCErrorToGin(ctx, err)
		return
	}

	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Domain:   utils.GetEnv("COOKIE_DOMAIN", ""),
		Path:     "/",
		MaxAge:   -1,
	})

	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     "device_id",
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Domain:   utils.GetEnv("COOKIE_DOMAIN", ""),
		Path:     "/",
		MaxAge:   -1,
	})

	utils.ResponseStatusCode(ctx, http.StatusNoContent)
}

func (h *AuthHandler) LogoutAll(ctx *gin.Context) {
	userID, exist := ctx.Get(middleware.CTX_USER_ID_KEY)
	if !exist {
		utils.ResponseErrorAbort(ctx, utils.NewError("user_id not found in context", utils.ErrCodeUnauthorized))
		return
	}

	id, ok := userID.(int32)
	if !ok {
		utils.ResponseErrorAbort(ctx, utils.NewError("user_id in context has invalid type", utils.ErrCodeInternal))
		return
	}

	if id <= 0 {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(errors.New("user_id must be greater than 0")))
		return
	}

	req := &auth_proto.LogoutAllRequest{
		UserId: id,
	}

	_, err := h.auth_client.Client.LogoutAll(ctx, req)
	if err != nil {
		utils.WriteGRPCErrorToGin(ctx, err)
		return
	}

	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Domain:   utils.GetEnv("COOKIE_DOMAIN", ""),
		Path:     "/",
		MaxAge:   -1,
	})

	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     "device_id",
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Domain:   utils.GetEnv("COOKIE_DOMAIN", ""),
		Path:     "/",
		MaxAge:   -1,
	})

	utils.ResponseStatusCode(ctx, http.StatusNoContent)
}
