package gortsplib

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/mikey"
	"github.com/bluenviron/gortsplib/v5/pkg/ntp"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/srtp/v3"
)

func mikeyGetPayload[T mikey.Payload](mikeyMsg *mikey.Message) (T, bool) {
	var zero T
	for _, wrapped := range mikeyMsg.Payloads {
		if val, ok := wrapped.(T); ok {
			return val, true
		}
	}
	return zero, false
}

func mikeyGetSPPolicy(spPayload *mikey.PayloadSP, typ mikey.PayloadSPPolicyParamType) ([]byte, bool) {
	for _, pl := range spPayload.PolicyParams {
		if pl.Type == typ {
			return pl.Value, true
		}
	}
	return nil, false
}

func mikeyToContext(mikeyMsg *mikey.Message) (*wrappedSRTPContext, error) {
	timePayload, ok := mikeyGetPayload[*mikey.PayloadT](mikeyMsg)
	if !ok {
		return nil, fmt.Errorf("time payload not present")
	}

	ts := ntp.Decode(timePayload.TSValue)
	diff := time.Since(ts)
	if diff < -time.Hour || diff > time.Hour {
		return nil, fmt.Errorf("NTP difference is too high")
	}

	spPayload, ok := mikeyGetPayload[*mikey.PayloadSP](mikeyMsg)
	if !ok {
		return nil, fmt.Errorf("SP payload not present")
	}

	v, ok := mikeyGetSPPolicy(spPayload, mikey.PayloadSPPolicyParamTypeEncrAlg)
	if !ok || !bytes.Equal(v, []byte{1}) {
		return nil, fmt.Errorf("missing or unsupported policy: PayloadSPPolicyParamTypeEncrAlg")
	}

	v, ok = mikeyGetSPPolicy(spPayload, mikey.PayloadSPPolicyParamTypeSessionEncrKeyLen)
	if !ok || !bytes.Equal(v, []byte{16}) {
		return nil, fmt.Errorf("missing or unsupported policy: PayloadSPPolicyParamTypeSessionEncrKeyLen")
	}

	v, ok = mikeyGetSPPolicy(spPayload, mikey.PayloadSPPolicyParamTypeAuthAlg)
	if !ok || !bytes.Equal(v, []byte{1}) {
		return nil, fmt.Errorf("missing or unsupported policy: PayloadSPPolicyParamTypeAuthAlg")
	}

	v, ok = mikeyGetSPPolicy(spPayload, mikey.PayloadSPPolicyParamTypeSRTPEncrOffOn)
	if !ok || !bytes.Equal(v, []byte{1}) {
		return nil, fmt.Errorf("missing or unsupported policy: PayloadSPPolicyParamTypeSRTPEncrOffOn")
	}

	v, ok = mikeyGetSPPolicy(spPayload, mikey.PayloadSPPolicyParamTypeSRTCPEncrOffOn)
	if !ok || !bytes.Equal(v, []byte{1}) {
		return nil, fmt.Errorf("missing or unsupported policy: PayloadSPPolicyParamTypeSRTCPEncrOffOn")
	}

	v, ok = mikeyGetSPPolicy(spPayload, mikey.PayloadSPPolicyParamTypeSRTPAuthOffOn)
	if !ok || !bytes.Equal(v, []byte{1}) {
		return nil, fmt.Errorf("missing or unsupported policy: PayloadSPPolicyParamTypeSRTPAuthOffOn")
	}

	kemacPayload, ok := mikeyGetPayload[*mikey.PayloadKEMAC](mikeyMsg)
	if !ok {
		return nil, fmt.Errorf("KEMAC payload not present")
	}

	if len(kemacPayload.SubPayloads) != 1 {
		return nil, fmt.Errorf("multiple keys are present")
	}

	if len(kemacPayload.SubPayloads[0].KeyData) != srtpKeyLength {
		return nil, fmt.Errorf("unexpected key size: %d", len(kemacPayload.SubPayloads[0].KeyData))
	}

	ssrcs := make([]uint32, len(mikeyMsg.Header.CSIDMapInfo))
	startROCs := make([]uint32, len(mikeyMsg.Header.CSIDMapInfo))

	for i, entry := range mikeyMsg.Header.CSIDMapInfo {
		ssrcs[i] = entry.SSRC
		startROCs[i] = entry.ROC
	}

	srtpCtx := &wrappedSRTPContext{
		key:       kemacPayload.SubPayloads[0].KeyData,
		ssrcs:     ssrcs,
		startROCs: startROCs,
	}
	err := srtpCtx.initialize()
	if err != nil {
		return nil, err
	}

	return srtpCtx, nil
}

func mikeyGenerate(ctx *wrappedSRTPContext) (*mikey.Message, error) {
	csbID, err := randUint32()
	if err != nil {
		return nil, err
	}

	msg := &mikey.Message{
		Header: mikey.Header{
			Version: 1,
			CSBID:   csbID,
		},
	}

	msg.Header.CSIDMapInfo = make([]mikey.SRTPIDEntry, len(ctx.ssrcs))

	n := 0
	for _, ssrc := range ctx.ssrcs {
		msg.Header.CSIDMapInfo[n] = mikey.SRTPIDEntry{
			PolicyNo: 0,
			SSRC:     ssrc,
			ROC:      ctx.roc(ssrc),
		}
		n++
	}

	randData := make([]byte, 16)
	_, err = rand.Read(randData)
	if err != nil {
		return nil, err
	}

	keyLen, err := ctx.profile.KeyLen()
	if err != nil {
		return nil, err
	}

	authKeyLen, err := ctx.profile.AuthKeyLen()
	if err != nil {
		return nil, err
	}

	authTagLen, err := ctx.profile.AuthTagRTPLen()
	if err != nil {
		return nil, err
	}

	msg.Payloads = []mikey.Payload{
		&mikey.PayloadT{
			TSType:  0,
			TSValue: ntp.Encode(time.Now()),
		},
		&mikey.PayloadRAND{
			Data: randData,
		},
		&mikey.PayloadSP{
			PolicyParams: []mikey.PayloadSPPolicyParam{
				{
					Type:  mikey.PayloadSPPolicyParamTypeEncrAlg,
					Value: []byte{1},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeSessionEncrKeyLen,
					Value: []byte{byte(keyLen)},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeAuthAlg,
					Value: []byte{1},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeSessionAuthKeyLen,
					Value: []byte{byte(authKeyLen)},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeSRTPEncrOffOn,
					Value: []byte{1},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeSRTCPEncrOffOn,
					Value: []byte{1},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeSRTPAuthOffOn,
					Value: []byte{1},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeAuthTagLen,
					Value: []byte{byte(authTagLen)},
				},
			},
		},
		&mikey.PayloadKEMAC{
			SubPayloads: []*mikey.SubPayloadKeyData{
				{
					Type:    mikey.SubPayloadKeyDataKeyTypeTEK,
					KeyData: ctx.key,
				},
			},
		},
	}

	return msg, nil
}

// srtp.Context with
// - accessible key
// - accessible SSRCs
// - mutex around Encrypt*, ROC*
type wrappedSRTPContext struct {
	key       []byte
	ssrcs     []uint32
	startROCs []uint32

	profile srtp.ProtectionProfile
	w       *srtp.Context
	mutex   sync.RWMutex
}

func (ctx *wrappedSRTPContext) initialize() error {
	ctx.profile = srtp.ProtectionProfileAes128CmHmacSha1_80

	var err error
	ctx.w, err = srtp.CreateContext(ctx.key[:16], ctx.key[16:], ctx.profile)
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
