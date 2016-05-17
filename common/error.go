package common

type ErrorHandler struct {
	Err error
}

func (e ErrorHandler) Do(f func()) {
	if e.Err == nil {
		f()
	}
}

func (e ErrorHandler) IfErr(f func()) {
	if e.Err != nil {
		f()
	}
}
