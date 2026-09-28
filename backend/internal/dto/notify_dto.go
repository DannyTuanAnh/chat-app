package dto

type RegisterDeviceRequest struct {
	PushToken string `json:"push_token" binding:"required,min=1"`
	UserID    int32  `json:"user_id" binding:"required,gt=0"`
	DeviceID  string `json:"device_id" binding:"required,uuid"`
}
