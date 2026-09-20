package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"time"

	"buf.build/go/protovalidate"
	"cloud.google.com/go/auth/credentials/idtoken"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/auth"
	auth_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/auth"
	user_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/user"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/models"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/repository"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type authService struct {
	auth_proto.UnimplementedAuthServiceServer
	auth_repo    repository.AuthRepository
	user_client  *client.UserClient
	redis_memory *redis.Client
	validator    protovalidate.Validator
}

func NewAuthService(auth_repo repository.AuthRepository, user_client *client.UserClient, rdb *redis.Client) *authService {
	v, err := protovalidate.New()
	if err != nil {
		panic(fmt.Sprintf("Failed to create validator: %v", err))
	}

	return &authService{
		auth_repo:    auth_repo,
		user_client:  user_client,
		redis_memory: rdb,
		validator:    v,
	}
}

func (s *authService) LoginGoogle(ctx context.Context, req *auth_proto.LoginRequest) (*auth_proto.LoginResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	deviceID, err := uuid.Parse(req.DeviceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to parse device ID: %v", err)
	}

	tokenResp, err := s.ExchangeGoogleCode(req.AuthorCode)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to exchange Google code: %v", err)
	}

	userInfo, err := s.VerifyIDGoogleToken(ctx, tokenResp.IdToken, tokenResp.AccessToken)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to verify Google ID token: %v", err)
	}

	name := fmt.Sprintf("%s %s", userInfo.Claims["family_name"].(string), userInfo.Claims["given_name"].(string))

	var userID int32

	respIsExistingIdentity, err := s.auth_repo.IsExistingIdentityID(ctx, sqlc.FindExistingIdentityParams{
		Provider:       "google",
		ProviderUserID: userInfo.Claims["sub"].(string),
	})

	if err != nil {
		if !errors.Is(err, repository.ErrNotFoundIdentityID) {
			return nil, status.Errorf(codes.Internal, "Failed to check existing identity: %v", err)
		}

		respCreateUser, err := s.user_client.Client.CreateUser(ctx, &user_proto.CreateUserRequest{
			DisplayName: name,
		})
		if err != nil {
			return nil, validation.MapServiceError(err, "user")
		}

		err = s.auth_repo.CreateIdentity(ctx, sqlc.CreateIdentityParams{
			UserID:         respCreateUser.UserId,
			Provider:       "google",
			ProviderUserID: userInfo.Claims["sub"].(string),
			Email:          utils.ConvertToPgTypeText(userInfo.Claims["email"].(string)),
		})
		if err != nil {
			if !errors.Is(err, repository.ErrIdentityAlreadyExist) {
				return nil, status.Errorf(codes.Internal, "Failed to create identity: %v", err)
			}

			//Compensating Action if identity creation fails, delete the user that was just created
			_, err := s.user_client.Client.DeleteUserByUserID(ctx, &user_proto.DeleteUserRequest{
				UserId: respCreateUser.UserId,
			})
			if err != nil {
				return nil, validation.MapServiceError(err, "user")
			}

			respIsExistingIdentity, err := s.auth_repo.IsExistingIdentityID(ctx, sqlc.FindExistingIdentityParams{
				Provider:       "google",
				ProviderUserID: userInfo.Claims["sub"].(string),
			})
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Failed to check existing identity after create identity failure by unique constraint: %v", err)
			}

			userID = respIsExistingIdentity.UserID
		} else {
			userID = respCreateUser.UserId
		}

		respIsExist, err := s.user_client.Client.IsExistProfile(ctx, &user_proto.IsExistProfileRequest{
			UserId: userID,
		})
		if err != nil {
			return nil, validation.MapServiceError(err, "user")
		}

		if !respIsExist.Exists {
			_, err := s.user_client.Client.CreateProfile(ctx, &user_proto.CreateProfileRequest{
				UserId:    userID,
				Name:      name,
				Email:     userInfo.Claims["email"].(string),
				Birthday:  userInfo.Claims["birthday"].(string),
				AvatarUrl: userInfo.Claims["picture"].(string),
			})
			if err != nil {
				return nil, validation.MapServiceError(err, "user")
			}
		}

	} else {
		userID = respIsExistingIdentity.UserID
	}

	if respIsExistingIdentity.Status == "revoked" && respIsExistingIdentity.RevokedAt.Valid && time.Since(respIsExistingIdentity.RevokedAt.Time) <= 30*24*time.Hour {
		_, err := s.user_client.Client.ActiveUser(ctx, &user_proto.EnableUserRequest{
			UserId: userID,
		})
		if err != nil {
			return nil, validation.MapServiceError(err, "user")
		}

		err = s.auth_repo.ActiveIdentity(ctx, sqlc.ActiveIdentityParams{
			Provider:       "google",
			ProviderUserID: userInfo.Claims["sub"].(string),
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to activate identity: %v", err)
		}
	} else if respIsExistingIdentity.Status == "revoked" && respIsExistingIdentity.RevokedAt.Valid && time.Since(respIsExistingIdentity.RevokedAt.Time) > 30*24*time.Hour {
		respCreateUser, err := s.user_client.Client.CreateUser(ctx, &user_proto.CreateUserRequest{
			DisplayName: name,
		})
		if err != nil {
			return nil, validation.MapServiceError(err, "user")
		}

		userID = respCreateUser.UserId

		err = s.auth_repo.CreateIdentity(ctx, sqlc.CreateIdentityParams{
			UserID:         userID,
			Provider:       "google",
			ProviderUserID: userInfo.Claims["sub"].(string),
			Email:          utils.ConvertToPgTypeText(userInfo.Claims["email"].(string)),
		})
		if err != nil {
			if !errors.Is(err, repository.ErrIdentityAlreadyExist) {
				return nil, status.Errorf(codes.Internal, "Failed to create identity: %v", err)
			}

			//Compensating Action if identity creation fails, delete the user that was just created
			_, err := s.user_client.Client.DeleteUserByUserID(ctx, &user_proto.DeleteUserRequest{
				UserId: userID,
			})
			if err != nil {
				return nil, validation.MapServiceError(err, "user")
			}

			respIsExistingIdentity, err := s.auth_repo.IsExistingIdentityID(ctx, sqlc.FindExistingIdentityParams{
				Provider:       "google",
				ProviderUserID: userInfo.Claims["sub"].(string),
			})
			if err != nil {
				return nil, status.Errorf(codes.Internal, "Failed to check existing identity after create identity failure by unique constraint: %v", err)
			}
			userID = respIsExistingIdentity.UserID
		}
		respIsExist, err := s.user_client.Client.IsExistProfile(ctx, &user_proto.IsExistProfileRequest{
			UserId: userID,
		})
		if err != nil {
			return nil, validation.MapServiceError(err, "user")
		}

		if !respIsExist.Exists {

			_, err := s.user_client.Client.CreateProfile(ctx, &user_proto.CreateProfileRequest{
				UserId:    userID,
				Name:      name,
				Email:     userInfo.Claims["email"].(string),
				Birthday:  userInfo.Claims["birthday"].(string),
				AvatarUrl: userInfo.Claims["picture"].(string),
			})
			if err != nil {
				return nil, validation.MapServiceError(err, "user")
			}
		}
	}

	sessionVersion, err := s.redis_memory.Get(ctx, fmt.Sprintf("user:%d:session_version", userID)).Int()
	if err != nil {
		log.Println("Error in get session_version (in auth service layer): ", err)
	}

	if sessionVersion == 0 {
		if err := s.redis_memory.SetNX(ctx, fmt.Sprintf("user:%d:session_version", userID), 1, 0).Err(); err != nil {
			log.Println("Error in create session_version if redis didn't exist session_version before (in auth service layer): ", err)
		}

		sessionVersion, err = s.redis_memory.Get(ctx, fmt.Sprintf("user:%d:session_version", userID)).Int()
		if err != nil {
			log.Println("Error in get session_version after setNX (in auth service layer): ", err)
			sessionVersion = 1
		}
	}

	deviceVersion, err := s.redis_memory.Get(ctx, fmt.Sprintf("user:%d:device_version", userID)).Int()
	if err != nil {
		log.Println("Error in get device_version (in auth service layer): ", err)
	}

	if deviceVersion == 0 {
		if err := s.redis_memory.SetNX(ctx, fmt.Sprintf("user:%d:device_version", userID), 1, 0).Err(); err != nil {
			log.Println("Error in create device_version if redis didn't exist device_version before (in auth service layer): ", err)
		}

		deviceVersion, err = s.redis_memory.Get(ctx, fmt.Sprintf("user:%d:device_version", userID)).Int()
		if err != nil {
			log.Println("Error in get device_version after setNX (in auth service layer): ", err)
			deviceVersion = 1
		}
	}

	argCreateDevice := sqlc.CreateDeviceParams{
		UserID:   userID,
		DeviceID: deviceID,
	}

	err = s.auth_repo.CreateDevice(ctx, argCreateDevice)
	if err != nil {
		if errors.Is(err, repository.ErrCannotCreateDevice) {
			return nil, status.Errorf(codes.ResourceExhausted, "Device limit reached for user: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "Failed to create device: %v", err)
	}

	argCreateSession := sqlc.CreateSessionParams{
		UserID:   userID,
		DeviceID: deviceID,
	}

	session, err := s.auth_repo.CreateSession(ctx, argCreateSession)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create session: %v", err)
	}

	sessionRedis := models.SessionRedis{
		UserID:         userID,
		SessionVersion: sessionVersion,
		DeviceVersion:  deviceVersion,
		Valid:          true,
	}

	sessionJson, err := json.Marshal(sessionRedis)
	if err != nil {
		log.Println("Error in marshal session data (in auth service layer): ", err)
	}

	err = s.redis_memory.Set(ctx, fmt.Sprintf("session:%s:device_id:%s", session, deviceID), sessionJson, 24*7*time.Hour).Err()
	if err != nil {
		log.Println("Error in set session with marshal data in Redis (in auth service layer): ", err)
	}

	return &auth_proto.LoginResponse{
		Success: true,
		Session: session.String(),
		UserId:  userID,
	}, nil

}

func (s *authService) ExchangeGoogleCode(code string) (*models.GoogleTokenResponse, error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", utils.GetEnv("GOOGLE_CLIENT_ID", ""))
	data.Set("client_secret", utils.GetEnv("GOOGLE_CLIENT_SECRET", ""))
	data.Set("redirect_uri", utils.GetEnv("GOOGLE_REDIRECT_URI", ""))
	data.Set("grant_type", "authorization_code")

	resp, err := http.PostForm(utils.GetEnv("GOOGLE_TOKEN_URL", ""), data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result models.GoogleTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *authService) VerifyIDGoogleToken(ctx context.Context, idToken string, accessToken string) (*idtoken.Payload, error) {
	resp, err := http.Get("https://oauth2.googleapis.com/tokeninfo?id_token=" + idToken)
	if err != nil {
		return nil, fmt.Errorf("Failed to verify ID token: %v", err)
	}
	defer resp.Body.Close()

	payload, err := idtoken.Validate(ctx, idToken, utils.GetEnv("GOOGLE_CLIENT_ID", ""))
	if err != nil {
		return nil, fmt.Errorf("Failed to validate ID token: %v", err)
	}

	err = s.GetBirthday(payload, accessToken)
	if err != nil {
		log.Printf("Failed to get birthday: %v", err)
		return nil, err
	}

	return payload, nil
}

func (s *authService) GetBirthday(userInfo *idtoken.Payload, accessToken string) error {
	req, err := http.NewRequest("GET", "https://people.googleapis.com/v1/people/me?personFields=birthdays", nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var birthdayResp models.GoogleBirthdayResponse
	err = json.NewDecoder(resp.Body).Decode(&birthdayResp)
	if err != nil {
		return err
	}

	if len(birthdayResp.Birthdays) == 0 {
		userInfo.Claims["birthday"] = ""
		return nil
	}

	b := birthdayResp.Birthdays[0].Date
	userInfo.Claims["birthday"] = fmt.Sprintf("%04d-%02d-%02d", b.Year, b.Month, b.Day)

	return nil
}

func (s *authService) Logout(ctx context.Context, req *auth_proto.LogoutRequest) (*auth_proto.LogoutResponse, error) {
	err := s.validator.Validate(req)
	if err != nil {
		return nil, validation.BuildValidationError(err)
	}

	sessionId, err := uuid.Parse(req.SessionId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to parse session ID: %v", err)
	}

	deviceId, err := uuid.Parse(req.DeviceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to parse device ID: %v", err)
	}

	arg := sqlc.RevokeSessionAndDeviceParams{
		SessionID: sessionId,
		DeviceID:  deviceId,
	}

	err = s.auth_repo.Logout(ctx, arg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to logout: %v", err)
	}

	if err := s.redis_memory.Del(ctx, fmt.Sprintf("session:%s:device_id:%s", sessionId, deviceId)).Err(); err != nil {
		log.Println("Error in delete session in Redis (in auth service layer): ", err)
	}

	return &auth_proto.LogoutResponse{
		Success: true,
		Message: "Logout successfully",
	}, nil
}

func (s *authService) DisableIdentity(ctx context.Context, req *auth_proto.DisableIdentityRequest) (*auth_proto.DisableIdentityResponse, error) {
	err := s.validator.Validate(req)
	if err != nil {
		return nil, validation.BuildValidationError(err)
	}

	err = s.auth_repo.DisableIdentity(ctx, sqlc.DisableIdentityParams{
		Provider: "google",
		UserID:   req.UserId,
	})

	if err != nil {
		if errors.Is(err, repository.ErrNotFoundIdentityID) {
			return nil, status.Errorf(codes.NotFound, "Identity not found")
		}

		return nil, status.Errorf(codes.Internal, "Failed to disable identity: %v", err)
	}

	return &auth_proto.DisableIdentityResponse{
		Success: true,
		Message: "Identity disabled successfully",
	}, nil
}

func (s *authService) LogoutAll(ctx context.Context, req *auth_proto.LogoutAllRequest) (*auth_proto.LogoutAllResponse, error) {
	err := s.validator.Validate(req)
	if err != nil {
		return nil, validation.BuildValidationError(err)
	}

	err = s.auth_repo.LogoutAll(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to logout all sessions: %v", err)
	}

	if err := s.redis_memory.Incr(ctx, fmt.Sprintf("user:%d:session_version", req.UserId)).Err(); err != nil {
		log.Println("Error in increment session_version in Redis (in auth service layer): ", err)
	}

	return &auth_proto.LogoutAllResponse{
		Success: true,
		Message: "Logout all sessions successfully",
	}, nil
}
