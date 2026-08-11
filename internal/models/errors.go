package models

import "errors"

var (
	ErrNoRecord           = errors.New("models: no matching record")
	ErrInvalidCredentials = errors.New("models: invalid credentials")
	ErrDuplicateName      = errors.New("models: duplicate name")
	ErrAlreadyLinked      = errors.New("models: already linked")
	ErrSelfLink           = errors.New("models: cannot link a person to themselves")
)
