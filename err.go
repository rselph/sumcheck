package main

type myError struct {
	code myErrorCode
	info string
}

func (e *myError) Error() (output string) {
	switch e.code {
	case code_OK:
		output = "            ok"

	case code_SKIPPED:
		output = "       skipped"

	case code_NEW_SUM:
		output = "  new checksum"

	case code_BAD_SUM:
		output = "  BAD CHECKSUM"

	case code_NOT_FOUND:
		output = "file not found"

	case code_NEWER:
		output = "    file newer"

	case code_OTHER:
		output = "         error"
	}

	if e.info != "" {
		output += ": "
		output += e.info
	}
	return
}

func NewError(code myErrorCode, f *fileJob, info string) (err *myError) {
	if f != nil {
		err = &myError{code, f.Fpath + " " + info}
	} else {
		err = &myError{code, info}
	}
	return err
}

func WrapError(err error) (myerr *myError) {
	if err == nil {
		return nil
	}
	return &myError{code_OTHER, err.Error()}
}

type myErrorCode int

const (
	code_OK myErrorCode = iota
	code_SKIPPED
	code_NEW_SUM
	code_BAD_SUM
	code_NOT_FOUND
	code_NEWER
	code_OTHER
)
