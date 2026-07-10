package apperr

// Error is a typed sentinel carrying the HTTP status, machine-readable
// code, human-readable message, and optional structured details for one
// error path through the system.
//
// Sentinel errors are declared as package-level *Error values; they work
// with errors.Is/errors.As through pointer identity even when wrapped via
// fmt.Errorf("...: %w", err), identical to how plain errors.New sentinels
// behave.
type Error struct {
	HTTPStatus int
	Code       string
	Message    string
	Details    any
}

func (e *Error) Error() string { return e.Message }

// New constructs a new sentinel *Error.
func New(httpStatus int, code, message string) *Error {
	return &Error{
		HTTPStatus: httpStatus,
		Code:       code,
		Message:    message,
		Details:    nil,
	}
}

// WithDetails returns a clone of err with the given details attached.
// The original sentinel is left untouched so it remains safe to share.
func WithDetails(err *Error, details any) *Error {
	if err == nil {
		return nil
	}
	return &Error{
		HTTPStatus: err.HTTPStatus,
		Code:       err.Code,
		Message:    err.Message,
		Details:    details,
	}
}
