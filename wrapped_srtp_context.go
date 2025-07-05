package gortsplib

import (
	"sync"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/srtp/v3"
)

// srtp.Context with
// - accessible key
// - accessible SSRCs
// - mutex around Encrypt*, ROC*
type wrappedSRTPContext struct {
	key       []byte
	ssrcs     []uint32
	startROCs []uint32

	w     *srtp.Context
	mutex sync.RWMutex
}

func (ctx *wrappedSRTPContext) initialize() error {
	var err error
	ctx.w, err = srtp.CreateContext(ctx.key[:16], ctx.key[16:], srtp.ProtectionProfileAes128CmHmacSha1_80)
	if err != nil {
		return err
	}

	for i, roc := range ctx.startROCs {
		ctx.w.SetROC(ctx.ssrcs[i], roc)
	}

	return nil
}

func (ctx *wrappedSRTPContext) decryptRTP(dst []byte, encrypted []byte, header *rtp.Header) ([]byte, error) {
	return ctx.w.DecryptRTP(dst, encrypted, header)
}

func (ctx *wrappedSRTPContext) decryptRTCP(dst []byte, encrypted []byte, header *rtcp.Header) ([]byte, error) {
	return ctx.w.DecryptRTCP(dst, encrypted, header)
}

func (ctx *wrappedSRTPContext) encryptRTP(dst []byte, plaintext []byte, header *rtp.Header) ([]byte, error) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	return ctx.w.EncryptRTP(dst, plaintext, header)
}

func (ctx *wrappedSRTPContext) encryptRTCP(dst []byte, decrypted []byte, header *rtcp.Header) ([]byte, error) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	return ctx.w.EncryptRTCP(dst, decrypted, header)
}

func (ctx *wrappedSRTPContext) roc(ssrc uint32) uint32 {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	v, _ := ctx.w.ROC(ssrc)
	return v
}
