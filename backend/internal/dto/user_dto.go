package dto

import (
	"time"

	"github.com/google/uuid"
)

type GetProfileByUserID struct {
	ID int32 `uri:"id" binding:"required,min=1"`
}

type SearchUserByUUIDRequest struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

type SearchUserByUUIDResponse struct {
	UserID                 int32   `json:"user_id"`
	Name                   string  `json:"name"`
	AvatarUrl              *string `json:"avatar_url"`
	IsFriend               bool    `json:"is_friend"`
	FriendRequestDirection *string `json:"friend_request_direction"`
}

type GetProfileResponse struct {
	UserUUID  uuid.UUID `json:"user_uuid"`
	Name      *string   `json:"name"`
	Email     string    `json:"email"`
	Phone     *string   `json:"phone"`
	Birthday  *string   `json:"birthday"`
	AvatarUrl *string   `json:"avatar_url"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateUserRequest struct {
	Name     *string `form:"name" binding:"omitempty,min=1,max=100,not_blank"`
	Birthday *string `form:"birthday" binding:"omitempty,min=10,datetime"`
	Phone    *string `form:"phone" binding:"omitempty,is_phone_mobile"`
}

type VerifyIDTokenOTP struct {
	IDToken string `form:"id_token" binding:"required,not_blank"`
}

type ReportUserImageRequest struct {
	UUID string `json:"uuid" binding:"required,uuid"`
	Name string `json:"name" binding:"required,min=1,not_blank,uuid"`
}
