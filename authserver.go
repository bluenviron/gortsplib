package gortsplib

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// AuthServer is an object that helps a server validating the credentials of a client.
type AuthServer struct {
	nonce string
	realm string
	user  string
	pass  string
}

// NewAuthServer allocates an AuthServer.
func NewAuthServer(user string, pass string) *AuthServer {
	nonceByts := make([]byte, 16)
	rand.Read(nonceByts)
	nonce := hex.EncodeToString(nonceByts)

	return &AuthServer{
		nonce: nonce,
		realm: "IPCAM",
		user:  user,
		pass:  pass,
	}
}

// GenerateHeader generates the WWW-Authenticate header needed by a client to log in.
func (as *AuthServer) GenerateHeader() []string {
	return []string{fmt.Sprintf("Digest nonce=\"%s\", realm=\"%s\"", as.nonce, as.realm)}
}

// ValidateHeader validates the Authorization header sent by a client after receiving the
// WWW-Authenticate header provided by GenerateHeader().
func (as *AuthServer) ValidateHeader(header []string, method string, path string) error {
	if len(header) != 1 {
		return fmt.Errorf("Authorization header not provided")
	}

	auth, err := ReadHeaderAuth(header[0])
	if err != nil {
		return err
	}

	inNonce, ok := auth.Values["nonce"]
	if !ok {
		return fmt.Errorf("nonce not provided")
	}

	inRealm, ok := auth.Values["realm"]
	if !ok {
		return fmt.Errorf("realm not provided")
	}

	inUsername, ok := auth.Values["username"]
	if !ok {
		return fmt.Errorf("username not provided")
	}

	inUri, ok := auth.Values["uri"]
	if !ok {
		return fmt.Errorf("uri not provided")
	}

	inResponse, ok := auth.Values["response"]
	if !ok {
		return fmt.Errorf("response not provided")
	}

	if inNonce != as.nonce {
		return fmt.Errorf("wrong nonce")
	}

	if inRealm != as.realm {
		return fmt.Errorf("wrong realm")
	}

	if inUsername != as.user {
		return fmt.Errorf("wrong username")
	}

	if inUri != path {
		return fmt.Errorf("wrong uri")
	}

	ha1 := md5Hex(as.user + ":" + as.realm + ":" + as.pass)
	ha2 := md5Hex(method + ":" + path)
	response := md5Hex(ha1 + ":" + as.nonce + ":" + ha2)

	if inResponse != response {
		return fmt.Errorf("wrong response")
	}

	return nil
}
