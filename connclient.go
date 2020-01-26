package gortsplib

import (
	"bufio"
	"net"
	"strconv"
)

// ConnClient is a client-side RTSP connection.
type ConnClient struct {
	nconn       net.Conn
	br          *bufio.Reader
	bw          *bufio.Writer
	session     string
	cseqEnabled bool
	cseq        int
	authProv    *authClientProvider
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

// EnableCseq allows to automatically insert the CSeq header into every outgoing request.
// CSeq is incremented after every request.
func (c *ConnClient) EnableCseq() {
	c.cseqEnabled = true
}

// SetCredentials allows to automatically insert Authenticate header into every outgoing request.
// The content of the header is computed with the given user, password, realm and nonce.
func (c *ConnClient) SetCredentials(user string, pass string, realm string, nonce string) {
	c.authProv = newAuthClientProvider(user, pass, realm, nonce)
}

// WriteRequest writes a Request.
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
