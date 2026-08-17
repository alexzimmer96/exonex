package domain

type ErrorKind string

const (
	ErrorKindInvalidArgument    ErrorKind = "INVALID_ARGUMENT"
	ErrorKindNotFound           ErrorKind = "NOT_FOUND"
	ErrorKindAlreadyExists      ErrorKind = "ALREADY_EXISTS"
	ErrorKindPermissionDenied   ErrorKind = "PERMISSION_DENIED"
	ErrorKindFailedPrecondition ErrorKind = "FAILED_PRECONDITION"
	ErrorKindInternal           ErrorKind = "INTERNAL"
)

type Error struct {
	Kind    ErrorKind `json:"kind"`
	Message string    `json:"message"`
	Wrapped error     `json:"wrapped"`
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) Unwrap() error {
	return e.Wrapped
}
