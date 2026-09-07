package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"

	"cloud.google.com/go/storage"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/dto"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/middleware"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/models"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	friend_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/friend"
	user_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/user"
)

type UserHandler struct {
	user_client   *client.UserClient
	friend_client *client.FriendClient
	redis_memory  *redis.Client
	gcs_client    *storage.Client
}

func NewUserHandler(user_client *client.UserClient, friend_client *client.FriendClient, rdb *redis.Client, gcs_client *storage.Client) *UserHandler {
	return &UserHandler{
		user_client:   user_client,
		friend_client: friend_client,
		redis_memory:  rdb,
		gcs_client:    gcs_client,
	}
}

func (h *UserHandler) GetProfile(ctx *gin.Context) {
	userId, exist := ctx.Get(middleware.CTX_USER_ID_KEY)
	if !exist {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID not found in context", utils.ErrCodeNotFound))
		return
	}

	userID, ok := userId.(int32)
	if !ok {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID in context has invalid type", utils.ErrCodeInternal))
		return
	}

	if userID <= 0 {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(errors.New("UserID must greater than 0")))
		return
	}

	profileData, err := h.redis_memory.Get(ctx, fmt.Sprintf("user_profile:%d", userID)).Bytes()
	if err == nil && len(profileData) > 0 {
		var cachedProfile models.ProfileRedis
		if err := json.Unmarshal(profileData, &cachedProfile); err == nil {
			log.Println("Profile data found in Redis in api-gateway for user ID: ", userID)
			utils.ResponseSuccessWithData(ctx, http.StatusOK, cachedProfile)
			return
		}
	}

	data, err := h.user_client.Client.GetProfile(ctx, &user_proto.GetProfileRequest{UserId: userID})
	if err != nil {
		utils.WriteGRPCErrorToGin(ctx, err)
		return
	}

	userUUID, err := uuid.Parse(data.GetUuid())
	if err != nil {
		utils.ResponseErrorAbort(ctx, utils.NewError("Invalid UUID format from user service", utils.ErrCodeInternal))
		return
	}

	resp := dto.GetProfileResponse{
		UserUUID:  userUUID,
		Name:      data.Name,
		Email:     data.Email,
		Phone:     data.Phone,
		Birthday:  data.Birthday,
		AvatarUrl: data.AvatarUrl,
		UpdatedAt: data.UpdatedAt.AsTime(),
	}

	utils.ResponseSuccessWithData(ctx, http.StatusOK, resp)
}

func (h *UserHandler) GetProfileByUserID(ctx *gin.Context) {
	var params dto.GetProfileByUserID
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	currentUserID, exist := ctx.Get(middleware.CTX_USER_ID_KEY)
	if !exist {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID not found in context", utils.ErrCodeNotFound))
		return
	}

	currentUserIDInt, ok := currentUserID.(int32)
	if !ok {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID in context has invalid type", utils.ErrCodeInternal))
		return
	}

	if currentUserIDInt <= 0 {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(errors.New("UserID must greater than 0")))
		return
	}

	resp, err := h.user_client.Client.GetProfileByUserID(ctx, &user_proto.GetProfileByUserIDRequest{
		CurrentUserId: currentUserIDInt,
		TargetUserId:  params.ID,
	})
	if err != nil {
		utils.WriteGRPCErrorToGin(ctx, err)
		return
	}

	utils.ResponseSuccessWithData(ctx, http.StatusOK, resp)
}

func (h *UserHandler) SearchUserByUUID(ctx *gin.Context) {
	var params dto.SearchUserByUUIDRequest
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	currentUserID, exist := ctx.Get(middleware.CTX_USER_ID_KEY)
	if !exist {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID not found in context", utils.ErrCodeNotFound))
		return
	}

	currentUserIDInt, ok := currentUserID.(int32)
	if !ok {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID in context has invalid type", utils.ErrCodeInternal))
		return
	}

	if currentUserIDInt <= 0 {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(errors.New("UserID must greater than 0")))
		return
	}

	targetUserUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(errors.New("Invalid UUID format")))
		return
	}

	dataSearchUser, err := h.user_client.Client.SearchUserByUUID(ctx, &user_proto.SearchUserByUUIDRequest{
		CurrentUserId:  currentUserIDInt,
		TargetUserUuid: targetUserUUID.String(),
	})
	if err != nil {
		utils.WriteGRPCErrorToGin(ctx, err)
		return
	}

	dataRelationship, err := h.friend_client.Client.GetRelationship(ctx, &friend_proto.GetRelationshipRequest{
		CurrentUserId: currentUserIDInt,
		TargetUserId:  dataSearchUser.UserId,
	})
	if err != nil {
		utils.WriteGRPCErrorToGin(ctx, err)
		return
	}

	resp := dto.SearchUserByUUIDResponse{
		UserID:                 dataSearchUser.UserId,
		Name:                   dataSearchUser.Name,
		AvatarUrl:              dataSearchUser.AvatarUrl,
		FriendRequestDirection: dataRelationship.FriendRequestDirection,
		IsFriend:               dataRelationship.IsFriend,
	}

	utils.ResponseSuccessWithData(ctx, http.StatusOK, resp)
}

func (h *UserHandler) UpdateProfile(ctx *gin.Context) {
	var req dto.UpdateUserRequest

	if err := ctx.ShouldBind(&req); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	// Get user id from context (required for both image-only and full updates)
	userId, exist := ctx.Get("user_id")
	if !exist {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID not found in context", utils.ErrCodeNotFound))
		return
	}

	userID, ok := userId.(int32)
	if !ok {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID in context has invalid type", utils.ErrCodeInternal))
		return
	}

	if userID <= 0 {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(errors.New("UserID must greater than 0")))
		return
	}

	imageFile, err := ctx.FormFile("avatar_url")

	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			imageFile = nil
		} else {
			utils.ResponseErrorAbort(ctx, utils.NewError(fmt.Sprintf("Failed to get avatar file: %v", err), utils.ErrCodeBadRequest))
			return
		}
	}

	// track object name when we upload so we can return it for image-only updates
	var objName string
	if imageFile != nil {
		file, oName, err := utils.ValidateAndReturnObjNameImage(userID, imageFile)
		if err != nil {
			utils.ResponseErrorAbort(ctx, utils.NewError(fmt.Sprintf("Invalid avatar file: %v", err), utils.ErrCodeBadRequest))
			return
		}
		objName = oName

		contentType := imageFile.Header.Get("Content-Type")

		err = h.uploadToGCS(ctx, file, objName, contentType)
		if err != nil {
			utils.ResponseErrorAbort(ctx, utils.NewError(fmt.Sprintf("Failed to upload avatar to GCS: %v", err), utils.ErrCodeInternal))
			return
		}
	}

	// Determine whether we need to call the user service. If no profile fields are provided
	// (only avatar was uploaded), skip calling the user service.
	needUpdateUser := false
	if req.Name != nil || req.Birthday != nil || req.Phone != nil {
		needUpdateUser = true
	}

	// Only run phone verification checks when we will call the user service (i.e. updating profile fields)
	if needUpdateUser {
		var verifiedPhoneStr string
		if v, exist := ctx.Get("verified_phone"); exist {
			if s, ok := v.(string); ok {
				verifiedPhoneStr = s
			}
		}

		if req.Phone != nil && verifiedPhoneStr == "" {
			utils.ResponseErrorAbort(ctx, utils.NewError("Phone number provided but not verified", utils.ErrCodeBadRequest))
			return
		}

		if req.Phone != nil && verifiedPhoneStr != *req.Phone {
			utils.ResponseErrorAbort(ctx, utils.NewError("Provided phone number does not match verified phone number", utils.ErrCodeBadRequest))
			return
		}

		if req.Phone == nil && verifiedPhoneStr != "" {
			utils.ResponseErrorAbort(ctx, utils.NewError("Verified phone number exists but no phone number provided in request", utils.ErrCodeBadRequest))
			return
		}
	} else {
		// Return object name so client can use or request processed URL later
		utils.ResponseSuccessWithData(ctx, http.StatusOK, map[string]string{"avatar_object": objName})
		return
	}

	resp, err := h.user_client.Client.UpdateProfile(ctx, &user_proto.UpdateProfileRequest{
		UserId:   userID,
		Name:     req.Name,
		Birthday: req.Birthday,
		Phone:    req.Phone,
	})
	if err != nil {
		utils.WriteGRPCErrorToGin(ctx, err)
		return
	}

	utils.ResponseSuccessWithData(ctx, http.StatusOK, resp)
}

func (h *UserHandler) DisableUser(ctx *gin.Context) {
	userId, exist := ctx.Get(middleware.CTX_USER_ID_KEY)
	if !exist {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID not found in context", utils.ErrCodeNotFound))
	}

	userID, ok := userId.(int32)
	if !ok {
		utils.ResponseErrorAbort(ctx, utils.NewError("User ID in context has invalid type", utils.ErrCodeInternal))
	}

	if userID <= 0 {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(errors.New("UserID must greater than 0")))
	}

	_, err := h.user_client.Client.DisableUserByUserID(ctx, &user_proto.DisableUserRequest{UserId: userID})
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

func (h *UserHandler) VerifyIDTokenOTP(ctx *gin.Context) {
	var req dto.UpdateUserRequest

	if err := ctx.ShouldBind(&req); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		ctx.Abort()
		return
	}

	if req.Phone == nil {
		ctx.Next()
		return
	}

	var reqOTP dto.VerifyIDTokenOTP

	if err := ctx.ShouldBind(&reqOTP); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		ctx.Abort()
		return
	}

	resp, err := h.user_client.Client.VerifyIDTokenOTP(ctx, &user_proto.VerifyIDTokenOTPRequest{IdToken: reqOTP.IDToken})
	if err != nil {
		utils.WriteGRPCErrorToGin(ctx, err)
		ctx.Abort()
		return
	}

	if !resp.Valid {
		utils.ResponseErrorAbort(ctx, utils.NewError(resp.GetMessage(), utils.ErrCodeUnauthorized))
		return
	}

	ctx.Set("verified_phone", resp.GetVerifiedPhone())
	ctx.Next()
}

func (h *UserHandler) ReportUserImage(ctx *gin.Context) {
	var req dto.ReportUserImageRequest

	if err := ctx.ShouldBind(&req); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	resp, err := h.user_client.Client.ReportUserImage(ctx, &user_proto.ReportUserImageRequest{
		Uuid:   req.UUID,
		Bucket: utils.GetEnv("GOOGLE_CLOUD_STORAGE_BUCKET_PROCESSED", ""),
		Name:   req.Name,
	})
	if err != nil {
		utils.WriteGRPCErrorToGin(ctx, err)
		return
	}

	utils.ResponseSuccessWithMessage(ctx, http.StatusOK, resp.GetMessage())
}

func (h *UserHandler) uploadToGCS(ctx context.Context, file multipart.File, objectName string, contentType string) error {
	bucketName := utils.GetEnv("GOOGLE_CLOUD_STORAGE_BUCKET_RAW", "")
	wc := h.gcs_client.Bucket(bucketName).Object(objectName).NewWriter(ctx)

	wc.ObjectAttrs.CacheControl = "no-store, max-age=0"
	wc.ContentType = contentType

	if _, err := io.Copy(wc, file); err != nil {
		return fmt.Errorf("failed to copy file to GCS: %v", err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %v", err)
	}

	return nil
}
