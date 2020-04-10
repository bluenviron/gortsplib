package gortsplib

import (
	"bufio"
	"net"
	"strconv"
)

// ConnClient is a client-side RTSP connection.
type ConnClient struct {
	nconn    net.Conn
	br       *bufio.Reader
	bw       *bufio.Writer
	session  string
	curCseq  int
	authProv *AuthClient
}

// NewConnClient allocates a ConnClient.
func NewConnClient(nconn net.Conn) *ConnClient {
	return &ConnClient{
		nconn: nconn,
		br:    bufio.NewReaderSize(nconn, 4096),
		bw:    bufio.NewWriterSize(nconn, 4096),
	}
}

// NetConn returns the underlying new.Conn.
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
	c.authProv, err = NewAuthClient(wwwAuthenticateHeader, user, pass)
	return err
}

// WriteRequest writes a Request.
func (c *ConnClient) WriteRequest(req *Request) error {
	if c.session != "" {
		if req.Header == nil {
			req.Header = make(Header)
		}
		req.Header["Session"] = []string{c.session}
	}

	if c.authProv != nil {
		if req.Header == nil {
			req.Header = make(Header)
		}
		req.Header["Authorization"] = c.authProv.GenerateHeader(req.Method, req.Url)
	}

	// automatically insert cseq into every outgoing request
	if req.Header == nil {
		req.Header = make(Header)
	}
	c.curCseq += 1
	req.Header["CSeq"] = []string{strconv.FormatInt(int64(c.curCseq), 10)}

	return req.write(c.bw)
}

// ReadResponse reads a response.
func (c *ConnClient) ReadResponse() (*Response, error) {
	return readResponse(c.br)
}

// ReadInterleavedFrame reads an InterleavedFrame.
func (c *ConnClient) ReadInterleavedFrame() (*InterleavedFrame, error) {
	return readInterleavedFrame(c.br)
}

// WriteInterleavedFrame writes an InterleavedFrame.
func (c *ConnClient) WriteInterleavedFrame(frame *InterleavedFrame) error {
	return frame.write(c.bw)
}
