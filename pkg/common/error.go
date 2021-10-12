package common

type TripleError struct {
	msg         string
	stacksTrace string
	attachment  map[string]string
	code        int
}

func NewTripleError(msg string, code int, stacksTrace string, attachment map[string]string) *TripleError {
	return &TripleError{
		msg:         msg,
		code:        code,
		attachment:  attachment,
		stacksTrace: stacksTrace,
	}
}

func (e *TripleError) Error() string {
	return e.msg
}

func (e *TripleError) StacksTrace() string {
	return e.stacksTrace
}

func (e *TripleError) Attachment() map[string]string {
	return e.attachment
}

func (e *TripleError) Code() int {
	return e.code
}
