package wintray

// A suspended session has already released its capture source while retaining
// the prepared adapter. Explicit Stop must still remove that peripheral even
// when it arrives between two connection attempts.
func finishReconnectLoop(stop <-chan struct{}, err error, virtual bool, disconnect func() error) error {
	if err != nil || !virtual || disconnect == nil {
		return err
	}
	select {
	case <-stop:
		return disconnect()
	default:
		return err
	}
}
