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

	// (optional) a username that will be sent to the server when requested
	Username string

	// (optional) a password that will be sent to the server when requested
	Password string

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
func NewConnClient(conf ConnClientConf) (*ConnClient, error) {
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

	if conf.Username != "" && conf.Password == "" ||
		conf.Username == "" && conf.Password != "" {
		return nil, fmt.Errorf("username and password must be both provided")
	}

	return &ConnClient{
		conf: conf,
		br:   bufio.NewReaderSize(conf.NConn, conf.ReadBufferSize),
		bw:   bufio.NewWriterSize(conf.NConn, conf.WriteBufferSize),
	}, nil
}

// NetConn returns the underlying net.Conn.
func (c *ConnClient) NetConn() net.Conn {
	return c.conf.NConn
}

func (c *ConnClient) writeRequest(req *Request) error {
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
	return req.write(c.bw)
}

// WriteRequest writes a request and reads a response.
func (c *ConnClient) WriteRequest(req *Request) (*Response, error) {
	err := c.writeRequest(req)
	if err != nil {
		return nil, err
	}

	c.conf.NConn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	res, err := readResponse(c.br)
	if err != nil {
		return nil, err
	}

	// get session from response
	if sxRaw, ok := res.Header["Session"]; ok && len(sxRaw) == 1 {
		sx, err := ReadHeaderSession(sxRaw[0])
		if err != nil {
			return nil, fmt.Errorf("unable to parse session header: %s", err)
		}
		c.session = sx.Session
	}

	// setup authentication
	if res.StatusCode == StatusUnauthorized && c.conf.Username != "" && c.auth == nil {
		auth, err := NewAuthClient(res.Header["WWW-Authenticate"], c.conf.Username, c.conf.Password)
		if err != nil {
			return nil, fmt.Errorf("unable to setup authentication: %s", err)
		}
		c.auth = auth

		// send request again
		return c.WriteRequest(req)
	}

	return res, nil
}

// WriteRequestNoResponse writes a request and does not wait for a response.
func (c *ConnClient) WriteRequestNoResponse(req *Request) error {
	return c.writeRequest(req)
}

// ReadInterleavedFrameOrResponse reads an InterleavedFrame or a Response.
func (c *ConnClient) ReadInterleavedFrameOrResponse(frame *InterleavedFrame) (interface{}, error) {
	c.conf.NConn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	b, err := c.br.ReadByte()
	if err != nil {
		return nil, err
	}
	c.br.UnreadByte()

	if b == _INTERLEAVED_FRAME_MAGIC {
		err := frame.read(c.br)
		if err != nil {
			return nil, err
		}
		return frame, err
	}

	return readResponse(c.br)
}

// ReadInterleavedFrame reads an InterleavedFrame.
func (c *ConnClient) ReadInterleavedFrame(frame *InterleavedFrame) error {
	c.conf.NConn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	return frame.read(c.br)
}

// WriteInterleavedFrame writes an InterleavedFrame.
func (c *ConnClient) WriteInterleavedFrame(frame *InterleavedFrame) error {
	c.conf.NConn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	return frame.write(c.bw)
}
