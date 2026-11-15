package auth

// Method is an authentication method.
type Method int

// authentication methods.
const (
	MethodBasic Method = iota
	MethodDigestMD5
	MethodDigestSHA256
)

// VerifyMethod is a validation method.
//
// Deprecated: replaced by Method.
type VerifyMethod = Method

// validation methods.
//
// Deprecated: all replaced by Method*.
const (
	VerifyMethodBasic        = MethodBasic
	VerifyMethodDigestMD5    = MethodDigestMD5
	VerifyMethodDigestSHA256 = MethodDigestSHA256
)
