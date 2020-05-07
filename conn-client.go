package gortsplib

import (
	"bufio"
	"net"
	"strconv"
	"time"
)

// ConnClientConf allows to configure a ConnClient.
type ConnClientConf struct {
	// pre-existing TCP connection that will be wrapped
	NConn net.Conn

	// (optional) timeout for read requests.
	// It defaults to 5 seconds
	ReadTimeout time.Duration

	// (optional) timeout for write requests.
	// It defaults to 5 seconds
	WriteTimeout time.Duration

	// (optional) size of the read buffer.
	// It defaults to 4096 bytes
	ReadBufferSize int

	// (optional) size of the write buffer.
	// It defaults to 4096 bytes
	WriteBufferSize int
}

// ConnClient is a client-side RTSP connection.
type ConnClient struct {
	conf    ConnClientConf
	br      *bufio.Reader
	bw      *bufio.Writer
	session string
	curCSeq int
	auth    *AuthClient
}

// NewConnClient allocates a ConnClient. See ConnClientConf for the options.
func NewConnClient(conf ConnClientConf) *ConnClient {
	if conf.ReadTimeout == time.Duration(0) {
		conf.ReadTimeout = 5 * time.Second
	}
	if conf.WriteTimeout == time.Duration(0) {
		conf.WriteTimeout = 5 * time.Second
	}
	if conf.ReadBufferSize == 0 {
		conf.ReadBufferSize = 4096
	}
	if conf.WriteBufferSize == 0 {
		conf.WriteBufferSize = 4096
	}

	return &ConnClient{
		conf: conf,
		br:   bufio.NewReaderSize(conf.NConn, conf.ReadBufferSize),
		bw:   bufio.NewWriterSize(conf.NConn, conf.WriteBufferSize),
	}
}

// NetConn returns the underlying net.Conn.
func (c *ConnClient) NetConn() net.Conn {
	return c.conf.NConn
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

	c.conf.NConn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	err := req.write(c.bw)
	if err != nil {
		return nil, err
	}

	c.conf.NConn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	return readResponse(c.br)
}

// ReadInterleavedFrame reads an InterleavedFrame.
func (c *ConnClient) ReadInterleavedFrame() (*InterleavedFrame, error) {
	c.conf.NConn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	return readInterleavedFrame(c.br)
}

// WriteInterleavedFrame writes an InterleavedFrame.
func (c *ConnClient) WriteInterleavedFrame(frame *InterleavedFrame) error {
	c.conf.NConn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	return frame.write(c.bw)
}
