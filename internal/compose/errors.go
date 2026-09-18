package compose

type OverrideError struct {
	Err error
}

func (e *OverrideError) Error() string { return e.Err.Error() }
func (e *OverrideError) Unwrap() error { return e.Err }
