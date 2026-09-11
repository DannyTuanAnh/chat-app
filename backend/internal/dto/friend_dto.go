package dto

type SendFriendRequestRequest struct {
	TargetUserID int32 `json:"target_user_id" binding:"required,min=1"`
}

type RejectFriendRequestRequest struct {
	RequestID int32 `json:"request_id" binding:"required,min=1"`
}

type GetPendingFriendRequestsRequest struct {
	LastRequestID int32 `form:"last_request_id" binding:"omitempty,min=0"`
}

type GetSentFriendRequestsRequest struct {
	LastRequestID int32 `form:"last_request_id" binding:"omitempty,min=0"`
}
