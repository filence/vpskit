//go:build windows

package platform

type ProcessLock struct{}

func AcquireProcessLock(_ string) (*ProcessLock, error) {
	return &ProcessLock{}, nil
}

func (lock *ProcessLock) Close() error {
	return nil
}
