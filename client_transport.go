package gortsplib

// ClientTransport contains details about the client transport.
type ClientTransport struct {
	Conn ConnTransport
	// present only when SETUP has been called at least once.
	Session *SessionTransport
}
