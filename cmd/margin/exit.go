package main

type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e exitError) ExitCode() int {
	if e.code == 0 {
		return 1
	}
	return e.code
}
