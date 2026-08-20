package repository

import "errors"

var (
	ErrInvalidFilter = errors.New("invalid filter")
	ErrUnknownField  = errors.New("unknown field")
	ErrNotFound      = errors.New("not found")
)
