package domain

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrEmailAlreadyInUse   = errors.New("email already in use")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrForbidden           = errors.New("forbidden")
	ErrInvalidInput        = errors.New("invalid input")
	ErrSelfAction          = errors.New("self action not allowed")
)
