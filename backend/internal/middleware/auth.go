package middleware

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/auth"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/models"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type identity string

const (
	CTX_USER_ID_KEY    identity = "user_id"
	CTX_USER_UUID_KEY  identity = "user_uuid"
	CTX_SESSION_ID_KEY identity = "session_id"
)

func AuthMiddleware(db sqlc.Querier, rdb *redis.Client) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var userId int32
		var userUUID uuid.UUID

		sessionId, err, errCode := ValidateSession(ctx)
		if err != nil {
			utils.ResponseErrorAbort(ctx, utils.WrapError(err, "Failed to validate session", errCode))
			return
		}

		data, err := rdb.Get(ctx, fmt.Sprintf("session:%s", sessionId)).Bytes()
		if err == nil && len(data) > 0 {
			log.Println("Session found in Redis")
			var valueSession models.SessionRedis
			if err := json.Unmarshal(data, &valueSession); err != nil {
				utils.ResponseErrorAbort(ctx, utils.WrapError(err, "Failed to decode session from Redis", utils.ErrCodeInternal))
				return
			}

			version, err := rdb.Get(ctx, fmt.Sprintf("user:%d:session_version", valueSession.UserID)).Int()
			if err != nil {
				utils.ResponseErrorAbort(ctx, utils.WrapError(err, "Failed to get session version from Redis", utils.ErrCodeInternal))
				return
			}

			if !valueSession.Valid || version != valueSession.SessionVersion {
				utils.ResponseErrorAbort(ctx, utils.NewError("Invalid session", utils.ErrCodeUnauthorized))
				return
			}

			if valueSession.UserID == 0 {
				utils.ResponseErrorAbort(ctx, utils.NewError("Invalid session: user_id is 0", utils.ErrCodeUnauthorized))
				return
			}

			userId = valueSession.UserID

		} else if errors.Is(err, redis.Nil) {
			log.Println("Session not found in Redis, checking database...")

			result, err := db.CheckSession(ctx, sessionId)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					utils.ResponseErrorAbort(ctx, utils.NewError("Session not found", utils.ErrCodeUnauthorized))
					return
				}
				utils.ResponseErrorAbort(ctx, utils.WrapError(err, "Failed to check session in database", utils.ErrCodeInternal))
				return
			}

			if result.Revoked {
				utils.ResponseErrorAbort(ctx, utils.NewError("Session revoked", utils.ErrCodeUnauthorized))
				return
			}
			userId = result.UserID

			version, err := rdb.Get(ctx, fmt.Sprintf("user:%d:session_version", userId)).Int()
			if err != nil {
				log.Println("Error in get session_version (in service layer): ", err)
			}

			if version == 0 {
				if err := rdb.SetNX(ctx, fmt.Sprintf("user:%d:session_version", userId), 1, 0).Err(); err != nil {
					log.Println("Error in create session_version if redis didn't exist session_version before (in service layer): ", err)
				}
				version, err = rdb.Get(ctx, fmt.Sprintf("user:%d:session_version", userId)).Int()
				if err != nil {
					log.Println("Error in get session_version after setNX (in service layer): ", err)
					version = 1
				}
			}

			sessionRedis := models.SessionRedis{
				UserID:         userId,
				SessionVersion: version,
				Valid:          result.Revoked,
			}

			sessionBytes, err := json.Marshal(sessionRedis)
			if err != nil {
				log.Println("Error in marshal session data (in service layer): ", err)
			}

			err = rdb.Set(ctx, fmt.Sprintf("session:%d", result.UserID), sessionBytes, 24*7*time.Hour).Err()
			if err != nil {
				log.Println("Error in set session with marshal data in Redis (in service layer): ", err)
			}

		} else {
			utils.ResponseErrorAbort(ctx, utils.WrapError(err, "Failed to get session from Redis", utils.ErrCodeInternal))
			return
		}

		ctx.Set(CTX_USER_ID_KEY, userId)
		ctx.Set(CTX_USER_UUID_KEY, userUUID.String())
		ctx.Set(CTX_SESSION_ID_KEY, sessionId.String())

		ctx.Next()

	}
}

func ValidateSession(ctx *gin.Context) (uuid.UUID, error, utils.ErrCode) {
	sessionId, err := ctx.Cookie("session_id")
	if err != nil {
		return uuid.Nil, fmt.Errorf("Failed to get session from cookie: %v", err), utils.ErrCodeInternal
	}

	if sessionId == "" {
		return uuid.Nil, fmt.Errorf("session_id cookie is empty"), utils.ErrCodeUnauthorized
	}

	if !utils.CheckUUID(sessionId) {
		return uuid.Nil, fmt.Errorf("invalid session_id format"), utils.ErrCodeUnauthorized
	}

	sessionUUID, err := uuid.Parse(sessionId)
	if err != nil {
		return uuid.Nil, fmt.Errorf("Failed to parse session_id: %v", err), utils.ErrCodeInternal
	}

	return sessionUUID, nil, ""
}

func ValidateDeviceID(ctx *gin.Context) (uuid.UUID, error, utils.ErrCode) {
	deviceId, err := ctx.Cookie("device_id")
	if err != nil {
		return uuid.Nil, fmt.Errorf("Failed to get device_id from cookie: %v", err), utils.ErrCodeInternal
	}

	if deviceId == "" {
		return uuid.Nil, fmt.Errorf("device_id cookie is empty"), utils.ErrCodeUnauthorized
	}

	if !utils.CheckUUID(deviceId) {
		return uuid.Nil, fmt.Errorf("invalid device_id format"), utils.ErrCodeUnauthorized
	}

	deviceUUID, err := uuid.Parse(deviceId)
	if err != nil {
		return uuid.Nil, fmt.Errorf("Failed to parse device_id: %v", err), utils.ErrCodeInternal
	}

	return deviceUUID, nil, ""
}
