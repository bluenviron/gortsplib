package gortsplib

import (
	"bufio"
	"fmt"
	"strconv"
)

// StatusCode is a RTSP response status code.
type StatusCode int

const (
	StatusContinue                           StatusCode = 100
	StatusOK                                 StatusCode = 200
	StatusMovedPermanently                   StatusCode = 301
	StatusFound                              StatusCode = 302
	StatusSeeOther                           StatusCode = 303
	StatusNotModified                        StatusCode = 304
	StatusUseProxy                           StatusCode = 305
	StatusBadRequest                         StatusCode = 400
	StatusUnauthorized                       StatusCode = 401
	StatusPaymentRequired                    StatusCode = 402
	StatusForbidden                          StatusCode = 403
	StatusNotFound                           StatusCode = 404
	StatusMethodNotAllowed                   StatusCode = 405
	StatusNotAcceptable                      StatusCode = 406
	StatusProxyAuthRequired                  StatusCode = 407
	StatusRequestTimeout                     StatusCode = 408
	StatusGone                               StatusCode = 410
	StatusPreconditionFailed                 StatusCode = 412
	StatusRequestEntityTooLarge              StatusCode = 413
	StatusRequestURITooLong                  StatusCode = 414
	StatusUnsupportedMediaType               StatusCode = 415
	StatusParameterNotUnderstood             StatusCode = 451
	StatusNotEnoughBandwidth                 StatusCode = 453
	StatusSessionNotFound                    StatusCode = 454
	StatusMethodNotValidInThisState          StatusCode = 455
	StatusHeaderFieldNotValidForResource     StatusCode = 456
	StatusInvalidRange                       StatusCode = 457
	StatusParameterIsReadOnly                StatusCode = 458
	StatusAggregateOperationNotAllowed       StatusCode = 459
	StatusOnlyAggregateOperationAllowed      StatusCode = 460
	StatusUnsupportedTransport               StatusCode = 461
	StatusDestinationUnreachable             StatusCode = 462
	StatusDestinationProhibited              StatusCode = 463
	StatusDataTransportNotReadyYet           StatusCode = 464
	StatusNotificationReasonUnknown          StatusCode = 465
	StatusKeyManagementError                 StatusCode = 466
	StatusConnectionAuthorizationRequired    StatusCode = 470
	StatusConnectionCredentialsNotAccepted   StatusCode = 471
	StatusFailureToEstablishSecureConnection StatusCode = 472
	StatusInternalServerError                StatusCode = 500
	StatusNotImplemented                     StatusCode = 501
	StatusBadGateway                         StatusCode = 502
	StatusServiceUnavailable                 StatusCode = 503
	StatusGatewayTimeout                     StatusCode = 504
	StatusRTSPVersionNotSupported            StatusCode = 505
	StatusOptionNotSupported                 StatusCode = 551
	StatusProxyUnavailable                   StatusCode = 553
)

// Response is a RTSP response.
type Response struct {
	// numeric status code
	StatusCode StatusCode

	// status message
	StatusMessage string

	// map of header values
	Header Header

	// optional content
	Content []byte
}

func readResponse(br *bufio.Reader) (*Response, error) {
	res := &Response{}

	byts, err := readBytesLimited(br, ' ', 255)
	if err != nil {
		return nil, err
	}
	proto := string(byts[:len(byts)-1])

	if proto != _RTSP_PROTO {
		return nil, fmt.Errorf("expected '%s', got '%s'", _RTSP_PROTO, proto)
	}

	byts, err = readBytesLimited(br, ' ', 4)
	if err != nil {
		return nil, err
	}
	statusCodeStr := string(byts[:len(byts)-1])

	statusCode64, err := strconv.ParseInt(statusCodeStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("unable to parse status code")
	}
	res.StatusCode = StatusCode(statusCode64)

	byts, err = readBytesLimited(br, '\r', 255)
	if err != nil {
		return nil, err
	}
	res.StatusMessage = string(byts[:len(byts)-1])

	if len(res.StatusMessage) == 0 {
		return nil, fmt.Errorf("empty status")
	}

	err = readByteEqual(br, '\n')
	if err != nil {
		return nil, err
	}

	res.Header, err = readHeader(br)
	if err != nil {
		return nil, err
	}

	res.Content, err = readContent(br, res.Header)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (res *Response) write(bw *bufio.Writer) error {
	if res.StatusMessage == "" {
		if status, ok := statusMessages[res.StatusCode]; ok {
			res.StatusMessage = status
		}
	}

	_, err := bw.Write([]byte(_RTSP_PROTO + " " + strconv.FormatInt(int64(res.StatusCode), 10) + " " + res.StatusMessage + "\r\n"))
	if err != nil {
		return err
	}

	if len(res.Content) != 0 {
		res.Header["Content-Length"] = []string{strconv.FormatInt(int64(len(res.Content)), 10)}
	}

	err = res.Header.write(bw)
	if err != nil {
		return err
	}

	err = writeContent(bw, res.Content)
	if err != nil {
		return err
	}

	return bw.Flush()
}
