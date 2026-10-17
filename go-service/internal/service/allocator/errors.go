package allocator

import "errors"

var (
	ErrInvalidInput     = errors.New("invalid allocator input")
	ErrNotEnoughHours   = errors.New("not enough hours for the number of topics")
	ErrNothingToBalance = errors.New("every topic is locked and the hours do not match the total")
)
