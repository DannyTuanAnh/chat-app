package dto

type RequestLoginGoogle struct {
	AuthCode string `json:"auth_code" binding:"required"`
	DeviceID string `json:"device_id" binding:"required,uuid"`
}
