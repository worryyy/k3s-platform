package platformerr

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrReleaseLockHeld = errors.New("release lock held")
)
