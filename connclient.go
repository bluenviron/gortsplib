package gortsplib

import (
	"bufio"
	"net"
	"strconv"
)

type ConnClient struct {
	nconn       net.Conn
	bw          *bufio.Writer
	session     string
	cseqEnabled bool
	cseq        int
	authProv    *authClientProvider
}

func NewConnClient(nconn net.Conn) *ConnClient {
	return &ConnClient{
		nconn: nconn,
		bw:    bufio.NewWriterSize(nconn, _INTERLEAVED_FRAME_MAX_SIZE),
	}
}

func (c *ConnClient) NetConn() net.Conn {
	return c.nconn
}

func (c *ConnClient) SetSession(v string) {
	c.session = v
}

func (c *ConnClient) EnableCseq() {
	c.cseqEnabled = true
}

func (c *ConnClient) SetCredentials(user string, pass string, realm string, nonce string) {
	c.authProv = newAuthClientProvider(user, pass, realm, nonce)
}

func (c *ConnClient) WriteRequest(req *Request) error {
	if c.session != "" {
		if req.Header == nil {
			req.Header = make(Header)
		}
		req.Header["Session"] = []string{c.session}
	}
	if c.cseqEnabled {
		if req.Header == nil {
			req.Header = make(Header)
		}
		c.cseq += 1
		req.Header["CSeq"] = []string{strconv.FormatInt(int64(c.cseq), 10)}
	}
	if c.authProv != nil {
		if req.Header == nil {
			req.Header = make(Header)
		}
		req.Header["Authorization"] = []string{c.authProv.generateHeader(req.Method, req.Url)}
	}
	return req.write(c.nconn)
}

func (c *ConnClient) ReadResponse() (*Response, error) {
	return readResponse(c.nconn)
}

func (c *ConnClient) ReadInterleavedFrame() (*InterleavedFrame, error) {
	return readInterleavedFrame(c.nconn)
}

func (c *ConnClient) WriteInterleavedFrame(frame *InterleavedFrame) error {
	return frame.write(c.bw)
}
