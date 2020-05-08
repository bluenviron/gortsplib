package gortsplib

import (
	"bufio"
	"fmt"
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

// SetCredentials allows to automatically insert the Authenticate header into every outgoing request.
// The content of the header is computed with the given user, password, realm and nonce.
func (c *ConnClient) SetCredentials(wwwAuthenticateHeader []string, user string, pass string) error {
	var err error
	c.auth, err = NewAuthClient(wwwAuthenticateHeader, user, pass)
	return err
}

// WriteRequest writes a request and reads a response.
func (c *ConnClient) WriteRequest(req *Request) (*Response, error) {
	if req.Header == nil {
		req.Header = make(Header)
	}

	// insert session
	if c.session != "" {
		req.Header["Session"] = []string{c.session}
	}

	// insert auth
	if c.auth != nil {
		req.Header["Authorization"] = c.auth.GenerateHeader(req.Method, req.Url)
	}

	// insert cseq
	c.curCSeq += 1
	req.Header["CSeq"] = []string{strconv.FormatInt(int64(c.curCSeq), 10)}

	c.conf.NConn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	err := req.write(c.bw)
	if err != nil {
		return nil, err
	}

	c.conf.NConn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	res, err := readResponse(c.br)
	if err != nil {
		return nil, err
	}

	// get session from response
	if res.StatusCode == StatusOK {
		if sxRaw, ok := res.Header["Session"]; ok && len(sxRaw) == 1 {
			sx, err := ReadHeaderSession(sxRaw[0])
			if err != nil {
				return nil, fmt.Errorf("unable to parse session header: %s", err)
			}
			c.session = sx.Session
		}
	}

	return res, nil
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
