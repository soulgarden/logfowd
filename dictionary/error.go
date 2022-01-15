package dictionary

import "errors"

var ErrChannelOverflowed = errors.New("channel overflowed")

var ErrBadStatusCode = errors.New("bas status code")

var ErrChannelClosed = errors.New("channel closed")

var ErrInterfaceAssertion = errors.New("invalid interface assertion")
