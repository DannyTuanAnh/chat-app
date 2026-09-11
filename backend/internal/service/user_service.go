package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/storage"
	vision "cloud.google.com/go/vision/apiv1"
	auth_firebase "firebase.google.com/go/auth"

	"buf.build/go/protovalidate"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/client"
	sqlc "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/db/sqlc/user"
	auth_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/auth"
	user_proto "github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/gen/user"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/models"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/repository"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/utils"
	"github.com/DannyTuanAnh/end-to-end_encrypted_messaging_app/internal/validation"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/api/compute/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type userService struct {
	user_proto.UnimplementedUserServiceServer
	user_repo    repository.UserRepository
	redis_memory *redis.Client
	validator    protovalidate.Validator

	auth_client *client.AuthClient

	auth_client_firebase *auth_firebase.Client
	vision_client        *vision.ImageAnnotatorClient

	compute_service *compute.Service
	gcs_client      *storage.Client
}

func NewUserService(user_repo repository.UserRepository, rdb *redis.Client, auth_client *client.AuthClient, auth_client_firebase *auth_firebase.Client, vision_client *vision.ImageAnnotatorClient, compute_service *compute.Service, gcs_client *storage.Client) *userService {
	v, err := protovalidate.New()
	if err != nil {
		panic(fmt.Sprintf("Failed to create validator: %v", err))
	}
	return &userService{
		user_repo:            user_repo,
		validator:            v,
		redis_memory:         rdb,
		auth_client:          auth_client,
		auth_client_firebase: auth_client_firebase,
		vision_client:        vision_client,
		compute_service:      compute_service,
		gcs_client:           gcs_client,
	}
}

func (s *userService) CreateUser(ctx context.Context, req *user_proto.CreateUserRequest) (*user_proto.CreateUserResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	userID, err := s.user_repo.CreateUser(ctx, req.DisplayName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create user: %v", err)
	}

	return &user_proto.CreateUserResponse{
		UserId:  userID,
		Message: "User created successfully",
	}, nil
}

func (s *userService) ActiveUser(ctx context.Context, req *user_proto.EnableUserRequest) (*user_proto.EnableUserResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	err := s.user_repo.ActiveUser(ctx, req.UserId)
	if err != nil {
		if err == repository.ErrCannotRestoreUser {
			return nil, validation.BuildBusinessError("CANNOT_RESTORE_USER", "Cannot restore user, user has been disabled for more than 30 days.")
		}
	}

	return &user_proto.EnableUserResponse{
		Success: true,
		Message: "User activated successfully",
	}, nil
}

func (s *userService) IsExistProfile(ctx context.Context, req *user_proto.IsExistProfileRequest) (*user_proto.IsExistProfileResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		fmt.Printf("Validation error in IsExistProfile: %v\n", err)
		return nil, validation.BuildValidationError(err)
	}

	exists, err := s.user_repo.IsExistProfile(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to check if profile exists: %v", err)
	}
	if !exists {
		return &user_proto.IsExistProfileResponse{
			Exists:  false,
			Message: "Profile does not exist for the given user ID",
		}, nil
	}

	return &user_proto.IsExistProfileResponse{
		Exists:  true,
		Message: "Profile exists for the given user ID",
	}, nil
}

func (s *userService) DeleteUserByUserID(ctx context.Context, req *user_proto.DeleteUserRequest) (*user_proto.DeleteUserResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	err := s.user_repo.DeleteUserByUserID(ctx, req.UserId)
	if err != nil {
		if errors.Is(err, repository.ErrCannotDeleteUser) {
			return nil, validation.BuildBusinessError("USER_NOT_FOUND", "User not found with the given ID. Please check the user ID and try again.")
		}

		return nil, status.Errorf(codes.Internal, "Failed to delete user: %v", err)
	}

	return &user_proto.DeleteUserResponse{
		Success: true,
		Message: "User deleted successfully",
	}, nil
}

func (s *userService) GetProfile(ctx context.Context, req *user_proto.GetProfileRequest) (*user_proto.GetProfileResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	profileData, err := s.redis_memory.Get(ctx, fmt.Sprintf("user_profile:%d", req.UserId)).Bytes()
	if err == nil && len(profileData) > 0 {
		var cachedProfile models.ProfileRedis
		if err := json.Unmarshal(profileData, &cachedProfile); err == nil {
			log.Println("Profile data found in Redis in user-service for user ID: ", req.UserId)
			return &user_proto.GetProfileResponse{
				Uuid:      cachedProfile.UserUUID.String(),
				Name:      cachedProfile.Name,
				Email:     cachedProfile.Email,
				Phone:     cachedProfile.Phone,
				Birthday:  cachedProfile.Birthday,
				AvatarUrl: cachedProfile.AvatarUrl,
				UpdatedAt: timestamppb.New(cachedProfile.UpdatedAt),
			}, nil
		}
	}

	data, err := s.user_repo.GetProfile(ctx, req.UserId)
	if err != nil {
		if errors.Is(err, repository.ErrNotFoundProfile) {
			return nil, validation.BuildBusinessError("PROFILE_NOT_FOUND", err.Error())
		}

		return nil, status.Errorf(codes.Internal, "Failed to get profile: %v", err)
	}

	var avatar_url, birthday *string

	if data.AvatarUrl.String == "" {
		avatar_url = nil
	} else {
		v := fmt.Sprintf("%s?v=%d", data.AvatarUrl.String, data.AvatarVersion)
		avatar_url = &v
	}

	if data.Birthday.Valid {
		birthdayStr := data.Birthday.Time.Format("2006-01-02")
		birthday = &birthdayStr
	} else {
		birthday = nil
	}

	profileRedis := models.ProfileRedis{
		UserUUID:  data.Uuid,
		Name:      &data.Name,
		Email:     data.Email.String,
		Phone:     &data.Phone.String,
		AvatarUrl: avatar_url,
		Birthday:  birthday,
		UpdatedAt: data.UpdatedAt,
	}

	profileBytes, err := json.Marshal(profileRedis)
	if err != nil {
		log.Printf("Failed to marshal profile data (in user service layer): %v", err)
	} else {
		if err := s.redis_memory.Set(ctx, fmt.Sprintf("user_profile:%d", req.UserId), profileBytes, 24*7*time.Hour).Err(); err != nil {
			log.Printf("Failed to set profile data in Redis (in user service layer): %v", err)
		} else {
			log.Println("Profile data cached in Redis for user ID: ", req.UserId)
		}
	}

	return &user_proto.GetProfileResponse{
		Uuid:      data.Uuid.String(),
		Name:      &data.Name,
		Email:     data.Email.String,
		Phone:     &data.Phone.String,
		Birthday:  birthday,
		AvatarUrl: avatar_url,
		UpdatedAt: timestamppb.New(data.UpdatedAt),
	}, nil
}

func (s *userService) GetProfileByUserID(ctx context.Context, req *user_proto.GetProfileByUserIDRequest) (*user_proto.GetProfileByUserIDResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	request := sqlc.GetProfileByUserIdParams{
		CurrentUserID: req.CurrentUserId,
		TargetUserID:  req.TargetUserId,
	}

	data, err := s.user_repo.GetProfileByUserID(ctx, request)
	if err != nil {
		if errors.Is(err, repository.ErrNotFoundProfileByUserID) {
			return nil, validation.BuildBusinessError("PROFILE_NOT_FOUND", err.Error())
		}

		return nil, status.Errorf(codes.Internal, "Failed to get profile: %v", err)
	}

	var avatar_url, birthday *string

	if data.AvatarUrl.String == "" {
		avatar_url = nil
	} else {
		v := fmt.Sprintf("%s?v=%v", data.AvatarUrl.String, data.AvatarVersion)
		avatar_url = &v
	}

	if data.Birthday.Valid {
		birthdayStr := data.Birthday.Time.Format("2006-01-02")
		birthday = &birthdayStr
	} else {
		birthday = nil
	}

	return &user_proto.GetProfileByUserIDResponse{
		Name:      data.Name,
		Birthday:  birthday,
		AvatarUrl: avatar_url,
	}, nil

}

func (s *userService) GetProfileByUserIDs(ctx context.Context, req *user_proto.GetProfileByUserIDsRequest) (*user_proto.GetProfileByUserIDsResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	data, err := s.user_repo.GetProfileByUserIDs(ctx, req.UserIds)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to get profiles by user IDs: %v", err)
	}

	var profiles []*user_proto.GetProfileByUserIDsResponse_UserProfile
	for _, d := range data {

		var avatar_url *string
		if d.AvatarUrl.Valid {
			v := fmt.Sprintf("%s?v=%d", d.AvatarUrl.String, d.AvatarVersion)
			avatar_url = &v
		} else {
			avatar_url = nil
		}

		info := &user_proto.GetProfileByUserIDsResponse_UserProfile{
			UserId:    d.UserID,
			Name:      d.Name,
			AvatarUrl: avatar_url,
		}

		profiles = append(profiles, info)
	}

	return &user_proto.GetProfileByUserIDsResponse{
		Profiles: profiles,
	}, nil
}

func (s *userService) SearchUserByUUID(ctx context.Context, req *user_proto.SearchUserByUUIDRequest) (*user_proto.SearchUserByUUIDResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	targetUserUUID, err := uuid.Parse(req.TargetUserUuid)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid UUID format: %v", err)
	}

	data, err := s.user_repo.GetUserByUUID(ctx, targetUserUUID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFoundIdentityUUID) {
			return nil, validation.BuildBusinessError("USER_NOT_FOUND", err.Error())
		} else if errors.Is(err, repository.ErrUserIsUnActive) {
			return nil, validation.BuildBusinessError("USER_INACTIVE", err.Error())
		}

		return nil, status.Errorf(codes.Internal, "Failed to get user by UUID: %v", err)
	}

	if data.UserID == req.CurrentUserId {
		return nil, validation.BuildBusinessError("CANNOT_SEARCH_SELF", "Cannot search for yourself. Please provide a different UUID.")
	}

	var avatar_url *string

	if data.AvatarUrl.String == "" {
		avatar_url = nil
	} else {
		v := fmt.Sprintf("%s?v=%d", data.AvatarUrl.String, data.AvatarVersion.Int32)
		avatar_url = &v
	}

	return &user_proto.SearchUserByUUIDResponse{
		UserId:    data.UserID,
		Name:      data.Name.String,
		AvatarUrl: avatar_url,
	}, nil
}

func (s *userService) CreateProfile(ctx context.Context, req *user_proto.CreateProfileRequest) (*user_proto.CreateProfileResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return nil, validation.BuildValidationError(err)
	}

	if req.Birthday != "" {
		birthday, _ := time.Parse("2006-01-02", req.Birthday)

		today := time.Now().Truncate(24 * time.Hour)
		if birthday.After(today) {
			return nil, status.Errorf(codes.InvalidArgument, "Birthday cannot be in the future")
		}
	}

	_, err := s.user_repo.CreateProfile(ctx, sqlc.CreateProfileParams{
		UserID:    req.UserId,
		Name:      req.Name,
		Email:     utils.ConvertToPgTypeText(req.Email),
		AvatarUrl: utils.ConvertToPgTypeText(req.AvatarUrl),
		Birthday:  utils.ConvertToPgTypeDate(req.Birthday),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create profile: %v", err)
	}

	return &user_proto.CreateProfileResponse{
		Success: true,
		Message: "Profile created successfully",
	}, nil
}

func (s *userService) UpdateProfile(ctx context.Context, req *user_proto.UpdateProfileRequest) (*user_proto.UpdateProfileResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	var phone string
	var isPhone bool
	if req.Phone != nil {
		phone, isPhone = utils.IsPhoneNumber(req.Phone)
		if !isPhone {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid phone number format")
		}
	}

	profile, err := s.user_repo.UpdateProfile(ctx, sqlc.UpdateProfileByUserIdParams{
		UserID:   req.UserId,
		Name:     utils.ConvertToPgTypeTextPtr(req.Name),
		Birthday: utils.ConvertToPgTypeDatePtr(req.Birthday),
		Phone:    utils.ConvertToPgTypeText(phone),
	})

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, validation.BuildBusinessError("USER_NOT_FOUND", "User not found with the given ID. Please check the user ID and try again.")
		}
		return nil, status.Errorf(codes.Internal, "Failed to update profile: %v", err)
	}

	var avatar_url, birthday *string

	if profile.AvatarUrl.String == "" {
		avatar_url = nil
	} else {
		v := fmt.Sprintf("%s?v=%d", profile.AvatarUrl.String, profile.AvatarVersion)
		avatar_url = &v
	}

	if profile.Birthday.Valid {
		birthdayStr := profile.Birthday.Time.Format("2006-01-02")
		birthday = &birthdayStr
	} else {
		birthday = nil
	}

	profileRedis := models.ProfileRedis{
		UserUUID:  profile.Uuid,
		Name:      &profile.Name,
		Email:     profile.Email.String,
		Phone:     &profile.Phone.String,
		AvatarUrl: avatar_url,
		Birthday:  birthday,
	}

	profileBytes, err := json.Marshal(profileRedis)
	if err != nil {
		log.Printf("Failed to marshal profile data (in user service layer): %v", err)
	} else {
		if err := s.redis_memory.Set(ctx, fmt.Sprintf("user_profile:%d", req.UserId), profileBytes, 24*7*time.Hour).Err(); err != nil {
			log.Printf("Failed to set profile data in Redis (in user service layer): %v", err)
		} else {
			log.Println("Profile data cached in Redis for user ID: ", req.UserId)
		}
	}

	return &user_proto.UpdateProfileResponse{
		Message: "Profile updated successfully",
	}, nil
}

func (s *userService) DisableUserByUserID(ctx context.Context, req *user_proto.DisableUserRequest) (*user_proto.DisableUserResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	err := s.user_repo.DisableUserByUserID(ctx, req.UserId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, validation.BuildBusinessError("USER_NOT_FOUND", "User not found with the given ID. Please check the user ID and try again.")
		}
		return nil, status.Errorf(codes.Internal, "Failed to disable user: %v", err)
	}

	reqLogoutAll := &auth_proto.LogoutAllRequest{
		UserId: req.UserId,
	}

	_, err = s.auth_client.Client.LogoutAll(ctx, reqLogoutAll)
	if err != nil {
		return nil, validation.MapServiceError(err, "auth")
	}

	_, err = s.auth_client.Client.DisableIdentity(ctx, &auth_proto.DisableIdentityRequest{
		Provider: "google",
		UserId:   req.UserId,
	})
	if err != nil {
		return nil, validation.MapServiceError(err, "auth")
	}

	return &user_proto.DisableUserResponse{
		Success: true,
		Message: "User disabled successfully",
	}, nil
}

func (s *userService) VerifyIDTokenOTP(ctx context.Context, req *user_proto.VerifyIDTokenOTPRequest) (*user_proto.VerifyIDTokenOTPResponse, error) {
	if err := s.validator.Validate(req); err != nil {
		return nil, validation.BuildValidationError(err)
	}

	token, err := s.auth_client_firebase.VerifyIDTokenAndCheckRevoked(ctx, req.GetIdToken())
	if err != nil {
		if auth_firebase.IsIDTokenRevoked(err) {
			return &user_proto.VerifyIDTokenOTPResponse{
				Valid:   false,
				Message: "ID token has been revoked. Please try again.",
			}, nil
		}
		return nil, status.Errorf(codes.Unauthenticated, "Failed to verify ID token: %v", err)
	}

	phoneClaim, ok := token.Claims["phone_number"].(string)
	if !ok || phoneClaim == "" {
		return &user_proto.VerifyIDTokenOTPResponse{
			Valid:   false,
			Message: "ID token does not contain a valid phone number claim.",
		}, nil
	}

	err = s.auth_client_firebase.RevokeRefreshTokens(ctx, token.UID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to revoke refresh tokens: %v", err)
	}

	return &user_proto.VerifyIDTokenOTPResponse{
		Valid:         true,
		VerifiedPhone: phoneClaim,
	}, nil
}

func (s *userService) ReportUserImage(ctx context.Context, req *user_proto.ReportUserImageRequest) (*user_proto.ReportUserImageResponse, error) {
	// 1. Use Cloud Vision API to check the image
	image := vision.NewImageFromURI("gs://" + req.Bucket + "/" + req.Name)
	props, err := s.vision_client.DetectSafeSearch(ctx, image, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to analyze image with Vision API: %v", err)
	}

	// 2. Check if the image is likely to contain adult content, violence, medical content, or racy content
	// level: VERY_UNLIKELY, UNLIKELY, POSSIBLE, LIKELY, VERY_LIKELY <=> 1, 2, 3, 4, 5
	if props.Adult >= 2 || props.Violence >= 2 || props.Medical >= 2 || props.Racy >= 2 { // 2 is 'UNLIKELY' (have a slight possibility)
		log.Printf("Detecting image violates community standards... Adult: %s, Violence: %s, Medical: %s, Racy: %s \nDeleting file %s from bucket %s", props.Adult, props.Violence, props.Medical, props.Racy, req.Name, req.Bucket)

		// Deleting cache for image from CDN immediately if it violates community standards
		err := s.InvalidateCacheOfProcessedBucket(ctx, req.Name)
		if err != nil {
			log.Fatalf("Failed to invalidate cache for inappropriate image: %v", err)
		}

		// Delete image immediately if it violates community standards
		err = s.gcs_client.Bucket(req.Bucket).Object(req.Name).Delete(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to delete inappropriate image from GCS: %v", err)
		}

		return &user_proto.ReportUserImageResponse{
			Message: "Image reported and removed due to inappropriate content",
		}, nil
	}

	return &user_proto.ReportUserImageResponse{
		Message: "Image reported successfully and does not violate community standards",
	}, nil
}

func (s *userService) InvalidateCacheOfProcessedBucket(ctx context.Context, filePath string) error {
	op, err := s.compute_service.UrlMaps.InvalidateCache(utils.GetEnv("PROJECT_ID", ""), utils.GetEnv("PROJECT_BALANCER_NAME", ""), &compute.CacheInvalidationRule{
		Path: utils.GetEnv("PROJECT_PATH_BUCKET_PROCESSED", "") + filePath,
	}).Do()

	if err != nil {
		return err
	}

	log.Printf("Invalidating cache for file %s from CDN... Operation: %s", filePath, op.Name)

	return nil
}
