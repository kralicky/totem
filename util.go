package totem

import (
	"fmt"
	"time"
)

var ErrTimeout = fmt.Errorf("timed out")

func WaitErrOrTimeout(errC <-chan error, timeout time.Duration) error {
	select {
	case err := <-errC:
		return err
	case <-time.After(timeout):
		return ErrTimeout
	}
}
