package gortsplib

import (
	"bufio"
	"net"
	"strconv"
	"time"
)

// ConnClient is a client-side RTSP connection.
type ConnClient struct {
	nconn        net.Conn
	br           *bufio.Reader
	bw           *bufio.Writer
	readTimeout  time.Duration
	writeTimeout time.Duration
	session      string
	curCSeq      int
	auth         *AuthClient
}

// NewConnClient allocates a ConnClient.
func NewConnClient(nconn net.Conn, readTimeout time.Duration, writeTimeout time.Duration) *ConnClient {
	return &ConnClient{
		nconn:        nconn,
		br:           bufio.NewReaderSize(nconn, 4096),
		bw:           bufio.NewWriterSize(nconn, 4096),
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
	}
}

// NetConn returns the underlying net.Conn.
func (c *ConnClient) NetConn() net.Conn {
	return c.nconn
}

// SetSession sets a Session header that is automatically inserted into every outgoing request.
func (c *ConnClient) SetSession(v string) {
	c.session = v
}

// SetCredentials allows to automatically insert the Authenticate header into every outgoing request.
// The content of the header is computed with the given user, password, realm and nonce.
func (c *ConnClient) SetCredentials(wwwAuthenticateHeader []string, user string, pass string) error {
	var err error
	c.auth, err = NewAuthClient(wwwAuthenticateHeader, user, pass)
	return err
}

// WriteRequest writes a request and reads a response.
func (c *ConnClient) WriteRequest(req *Request) (*Response, error) {
	if c.session != "" {
		if req.Header == nil {
			req.Header = make(Header)
		}
		req.Header["Session"] = []string{c.session}
	}

	if c.auth != nil {
		if req.Header == nil {
			req.Header = make(Header)
		}
		req.Header["Authorization"] = c.auth.GenerateHeader(req.Method, req.Url)
	}

	// automatically insert CSeq
	if req.Header == nil {
		req.Header = make(Header)
	}
	c.curCSeq += 1
	req.Header["CSeq"] = []string{strconv.FormatInt(int64(c.curCSeq), 10)}

	c.nconn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	err := req.write(c.bw)
	if err != nil {
		return nil, err
	}

	c.nconn.SetReadDeadline(time.Now().Add(c.readTimeout))
	return readResponse(c.br)
}

// ReadInterleavedFrame reads an InterleavedFrame.
func (c *ConnClient) ReadInterleavedFrame() (*InterleavedFrame, error) {
	c.nconn.SetReadDeadline(time.Now().Add(c.readTimeout))
	return readInterleavedFrame(c.br)
}

// WriteInterleavedFrame writes an InterleavedFrame.
func (c *ConnClient) WriteInterleavedFrame(frame *InterleavedFrame) error {
	c.nconn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	return frame.write(c.bw)
}
