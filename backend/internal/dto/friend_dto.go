package dto

type SendFriendRequestRequest struct {
	TargetUserID int32 `json:"target_user_id" binding:"required,min=1"`
}
