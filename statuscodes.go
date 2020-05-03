package gortsplib

// StatusCode is a RTSP response status code.
type StatusCode int

const (
	StatusContinue StatusCode = 100

	StatusOK StatusCode = 200

	StatusMovedPermanently StatusCode = 301
	StatusFound            StatusCode = 302
	StatusSeeOther         StatusCode = 303
	StatusNotModified      StatusCode = 304
	StatusUseProxy         StatusCode = 305

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

	StatusInternalServerError     StatusCode = 500
	StatusNotImplemented          StatusCode = 501
	StatusBadGateway              StatusCode = 502
	StatusServiceUnavailable      StatusCode = 503
	StatusGatewayTimeout          StatusCode = 504
	StatusRTSPVersionNotSupported StatusCode = 505
	StatusOptionNotSupported      StatusCode = 551
	StatusProxyUnavailable        StatusCode = 553
)
