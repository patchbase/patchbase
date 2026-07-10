package ws

import "errors"

var (
	errInvalidAuthMessage         = errors.New("invalid auth message")
	errInvalidSubscriptionMessage = errors.New("invalid subscription message")
)
