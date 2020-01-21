package gortsplib

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strconv"
)

func md5Hex(in string) string {
	h := md5.New()
	h.Write([]byte(in))
	return hex.EncodeToString(h.Sum(nil))
}

type authProvider struct {
	user  string
	pass  string
	realm string
	nonce string
}

func newAuthProvider(user string, pass string, realm string, nonce string) *authProvider {
	return &authProvider{
		user:  user,
		pass:  pass,
		realm: realm,
		nonce: nonce,
	}
}

func (ap *authProvider) generateHeader(method string, path string) string {
	ha1 := md5Hex(ap.user + ":" + ap.realm + ":" + ap.pass)
	ha2 := md5Hex(method + ":" + path)
	response := md5Hex(ha1 + ":" + ap.nonce + ":" + ha2)

	return fmt.Sprintf("Digest username=\"%s\", realm=\"%s\", nonce=\"%s\", uri=\"%s\", response=\"%s\"",
		ap.user, ap.realm, ap.nonce, path, response)
}

type Conn struct {
	nconn       net.Conn
	writeBuf    []byte
	cseqEnabled bool
	cseq        int
	session     string
	authProv    *authProvider
}

func NewConn(nconn net.Conn) *Conn {
	return &Conn{
		nconn:    nconn,
		writeBuf: make([]byte, 2048),
	}
}

func (c *Conn) NetConn() net.Conn {
	return c.nconn
}

func (c *Conn) EnableCseq() {
	c.cseqEnabled = true
}

func (c *Conn) SetSession(v string) {
	c.session = v
}

func (c *Conn) SetCredentials(user string, pass string, realm string, nonce string) {
	c.authProv = newAuthProvider(user, pass, realm, nonce)
}

func (c *Conn) ReadRequest() (*Request, error) {
	return requestDecode(c.nconn)
}

func (c *Conn) WriteRequest(req *Request) error {
	if c.cseqEnabled {
		if req.Headers == nil {
			req.Headers = make(map[string]string)
		}
		c.cseq += 1
		req.Headers["CSeq"] = strconv.FormatInt(int64(c.cseq), 10)
	}
	if c.session != "" {
		if req.Headers == nil {
			req.Headers = make(map[string]string)
		}
		req.Headers["Session"] = c.session
	}
	if c.authProv != nil {
		if req.Headers == nil {
			req.Headers = make(map[string]string)
		}
		req.Headers["Authorization"] = c.authProv.generateHeader(req.Method, req.Url)
	}
	return requestEncode(c.nconn, req)
}

func (c *Conn) ReadResponse() (*Response, error) {
	return responseDecode(c.nconn)
}

func (c *Conn) WriteResponse(res *Response) error {
	return responseEncode(c.nconn, res)
}

func (c *Conn) ReadInterleavedFrame(buf []byte) (int, int, error) {
	var header [4]byte
	_, err := io.ReadFull(c.nconn, header[:])
	if err != nil {
		return 0, 0, err
	}

	// connection terminated
	if header[0] == 0x54 {
		return 0, 0, io.EOF
	}

	if header[0] != 0x24 {
		return 0, 0, fmt.Errorf("wrong magic byte (0x%.2x)", header[0])
	}

	framelen := binary.BigEndian.Uint16(header[2:])
	if int(framelen) > len(buf) {
		return 0, 0, fmt.Errorf("frame length greater than buffer length")
	}

	_, err = io.ReadFull(c.nconn, buf[:framelen])
	if err != nil {
		return 0, 0, err
	}

	return int(header[1]), int(framelen), nil
}

func (c *Conn) WriteInterleavedFrame(channel int, frame []byte) error {
	c.writeBuf[0] = 0x24
	c.writeBuf[1] = byte(channel)
	binary.BigEndian.PutUint16(c.writeBuf[2:], uint16(len(frame)))
	n := copy(c.writeBuf[4:], frame)

	_, err := c.nconn.Write(c.writeBuf[:4+n])
	if err != nil {
		return err
	}
	return nil
}
