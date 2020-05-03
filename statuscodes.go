package gortsplib

// StatusCode is a RTSP response status code.
type StatusCode int

// https://tools.ietf.org/html/rfc7826#section-17

const (
	StatusContinue StatusCode = 100

	StatusOK = 200

	StatusMovedPermanently = 301
	StatusFound            = 302
	StatusSeeOther         = 303
	StatusNotModified      = 304
	StatusUseProxy         = 305

	StatusBadRequest                         = 400
	StatusUnauthorized                       = 401
	StatusPaymentRequired                    = 402
	StatusForbidden                          = 403
	StatusNotFound                           = 404
	StatusMethodNotAllowed                   = 405
	StatusNotAcceptable                      = 406
	StatusProxyAuthRequired                  = 407
	StatusRequestTimeout                     = 408
	StatusGone                               = 410
	StatusPreconditionFailed                 = 412
	StatusRequestEntityTooLarge              = 413
	StatusRequestURITooLong                  = 414
	StatusUnsupportedMediaType               = 415
	StatusParameterNotUnderstood             = 451
	StatusNotEnoughBandwidth                 = 453
	StatusSessionNotFound                    = 454
	StatusMethodNotValidInThisState          = 455
	StatusHeaderFieldNotValidForResource     = 456
	StatusInvalidRange                       = 457
	StatusParameterIsReadOnly                = 458
	StatusAggregateOperationNotAllowed       = 459
	StatusOnlyAggregateOperationAllowed      = 460
	StatusUnsupportedTransport               = 461
	StatusDestinationUnreachable             = 462
	StatusDestinationProhibited              = 463
	StatusDataTransportNotReadyYet           = 464
	StatusNotificationReasonUnknown          = 465
	StatusKeyManagementError                 = 466
	StatusConnectionAuthorizationRequired    = 470
	StatusConnectionCredentialsNotAccepted   = 471
	StatusFailureToEstablishSecureConnection = 472

	StatusInternalServerError     = 500
	StatusNotImplemented          = 501
	StatusBadGateway              = 502
	StatusServiceUnavailable      = 503
	StatusGatewayTimeout          = 504
	StatusRTSPVersionNotSupported = 505
	StatusOptionNotSupported      = 551
	StatusProxyUnavailable        = 553
)
