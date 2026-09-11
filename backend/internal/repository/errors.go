package repository

import "errors"

var (
	// Auth Repository Errors
	ErrIdentityAlreadyExist      = errors.New("identity already exists")
	ErrNotFoundIdentityID        = errors.New("identity not found")
	ErrCannotRestoreUserIdentity = errors.New("cannot restore user identity, identity has been disabled for more than 30 days")
	ErrNotFoundSessionID         = errors.New("session not found")

	// User Repository Errors
	ErrCannotDeleteUser                  = errors.New("cannot delete user, user does not exist or has already been deleted")
	ErrCannotRestoreUser                 = errors.New("cannot restore user, user has been disabled for more than 30 days")
	ErrSameUser                          = errors.New("cannot perform action on the same user")
	ErrNotFoundProfile                   = errors.New("Something went wrong, your profile is not found, please contact support for assistance")
	ErrNotFoundProfileByUserID           = errors.New("profile not found for the given user ID")
	ErrNotFoundIdentityUUID              = errors.New("identity not found for the given UUID")
	ErrUserIsUnActive                    = errors.New("user is inactive for the given UUID")
	ErrNoRowsRejectFriendRequestAffected = errors.New("no rows affected when rejecting friend request, may be you are trying to reject a friend request that does not exist or has already been accepted or rejected")
)
