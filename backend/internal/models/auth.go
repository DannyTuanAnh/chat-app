package models

import "github.com/google/uuid"

type GoogleTokenResponse struct {
	AccessToken string `json:"access_token"`
	IdToken     string `json:"id_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Birthday      string `json:"birthday"`
	Picture       string `json:"picture"`
	EmailVerified any    `json:"email_verified"`
}

type GoogleLoginResponse struct {
	SessionId     uuid.UUID `json:"session_id"`
	UUID          uuid.UUID `json:"uuid"`
	UserId        int32     `json:"user_id"`
	ProfileExists bool      `json:"profile_exists"`
}

type GoogleBirthdayResponse struct {
	Birthdays []struct {
		Date struct {
			Year  int `json:"year"`
			Month int `json:"month"`
			Day   int `json:"day"`
		} `json:"date"`
	} `json:"birthdays"`
}

type SessionRedis struct {
	UserID         int32 `json:"user_id"`
	SessionVersion int   `json:"session_version"`
	Valid          bool  `json:"valid"`
}
