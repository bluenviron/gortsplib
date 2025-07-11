package gortsplib

import (
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
)

type serverSessionMedia struct {
	ss           *ServerSession
	media        *description.Media
	srtpInCtx    *wrappedSRTPContext
	onPacketRTCP OnPacketRTCPFunc

	srtpOutCtx             *wrappedSRTPContext
	tcpChannel             int
	udpRTPReadPort         int
	udpRTPWriteAddr        *net.UDPAddr
	udpRTCPReadPort        int
	udpRTCPWriteAddr       *net.UDPAddr
	formats                map[uint8]*serverSessionFormat // record only
	writePacketRTCPInQueue func([]byte) error
	bytesReceived          *uint64
	bytesSent              *uint64
	rtpPacketsInError      *uint64
	rtcpPacketsReceived    *uint64
	rtcpPacketsSent        *uint64
	rtcpPacketsInError     *uint64
}

func (sm *serverSessionMedia) initialize() error {
	sm.bytesReceived = new(uint64)
	sm.bytesSent = new(uint64)
	sm.rtpPacketsInError = new(uint64)
	sm.rtcpPacketsReceived = new(uint64)
	sm.rtcpPacketsSent = new(uint64)
	sm.rtcpPacketsInError = new(uint64)

	sm.formats = make(map[uint8]*serverSessionFormat)

	for _, forma := range sm.media.Formats {
		f := &serverSessionFormat{
			sm:          sm,
			format:      forma,
			onPacketRTP: func(*rtp.Packet) {},
		}
		err := f.initialize()
		if err != nil {
			return err
		}
		sm.formats[forma.PayloadType()] = f
	}

	if sm.ss.s.TLSConfig != nil {
		if sm.ss.state == ServerSessionStatePreRecord || sm.media.IsBackChannel {
			srtpOutKey := make([]byte, srtpKeyLength)
			_, err := rand.Read(srtpOutKey)
			if err != nil {
				return err
			}

			ssrcs := make([]uint32, len(sm.formats))
			n := 0
			for _, cf := range sm.formats {
				ssrcs[n] = cf.localSSRC
				n++
			}

			sm.srtpOutCtx = &wrappedSRTPContext{
				key:   srtpOutKey,
				ssrcs: ssrcs,
			}
			err = sm.srtpOutCtx.initialize()
			if err != nil {
				return err
			}
		} else {
			streamMedia := sm.ss.setuppedStream.medias[sm.media]
			sm.srtpOutCtx = streamMedia.srtpOutCtx
		}
	}

	return nil
}

func (sm *serverSessionMedia) start() error {
	// allocate udpRTCPReceiver before udpRTCPListener
	// otherwise udpRTCPReceiver.LastSSRC() cannot be called.
	for _, sf := range sm.formats {
		sf.start()
	}

	switch *sm.ss.setuppedTransport {
	case TransportUDP, TransportUDPMulticast:
		sm.writePacketRTCPInQueue = sm.writePacketRTCPInQueueUDP

		if *sm.ss.setuppedTransport == TransportUDP {
			if sm.ss.state == ServerSessionStatePlay {
				if sm.media.IsBackChannel {
					sm.ss.s.udpRTPListener.addClient(sm.ss.author.ip(), sm.udpRTPReadPort, sm.readPacketRTPUDPPlay)
				}
				sm.ss.s.udpRTCPListener.addClient(sm.ss.author.ip(), sm.udpRTCPReadPort, sm.readPacketRTCPUDPPlay)
			} else {
				// open the firewall by sending empty packets to the remote part.
				buf, _ := (&rtp.Packet{Header: rtp.Header{Version: 2}}).Marshal()
				if sm.srtpOutCtx != nil {
					encr := make([]byte, sm.ss.s.MaxPacketSize)
					encr, err := sm.srtpOutCtx.encryptRTP(encr, buf, nil)
					if err != nil {
						return err
					}
					buf = encr
				}
				err := sm.ss.s.udpRTPListener.write(buf, sm.udpRTPWriteAddr)
				if err != nil {
					return err
				}

				buf, _ = (&rtcp.ReceiverReport{}).Marshal()
				if sm.srtpOutCtx != nil {
					encr := make([]byte, sm.ss.s.MaxPacketSize)
					encr, err = sm.srtpOutCtx.encryptRTCP(encr, buf, nil)
					if err != nil {
						return err
					}
					buf = encr
				}
				err = sm.ss.s.udpRTCPListener.write(buf, sm.udpRTCPWriteAddr)
				if err != nil {
					return err
				}

				sm.ss.s.udpRTPListener.addClient(sm.ss.author.ip(), sm.udpRTPReadPort, sm.readPacketRTPUDPRecord)
				sm.ss.s.udpRTCPListener.addClient(sm.ss.author.ip(), sm.udpRTCPReadPort, sm.readPacketRTCPUDPRecord)
			}
		}

	case TransportTCP:
		sm.writePacketRTCPInQueue = sm.writePacketRTCPInQueueTCP

		if sm.ss.tcpCallbackByChannel == nil {
			sm.ss.tcpCallbackByChannel = make(map[int]readFunc)
		}

		if sm.ss.state == ServerSessionStatePlay {
			sm.ss.tcpCallbackByChannel[sm.tcpChannel] = sm.readPacketRTPTCPPlay
			sm.ss.tcpCallbackByChannel[sm.tcpChannel+1] = sm.readPacketRTCPTCPPlay
		} else {
			sm.ss.tcpCallbackByChannel[sm.tcpChannel] = sm.readPacketRTPTCPRecord
			sm.ss.tcpCallbackByChannel[sm.tcpChannel+1] = sm.readPacketRTCPTCPRecord
		}
	}

	return nil
}

func (sm *serverSessionMedia) stop() {
	if *sm.ss.setuppedTransport == TransportUDP {
		sm.ss.s.udpRTPListener.removeClient(sm.ss.author.ip(), sm.udpRTPReadPort)
		sm.ss.s.udpRTCPListener.removeClient(sm.ss.author.ip(), sm.udpRTCPReadPort)
	}

	for _, sf := range sm.formats {
		sf.stop()
	}
}

func (sm *serverSessionMedia) findFormatByRemoteSSRC(ssrc uint32) *serverSessionFormat {
	for _, format := range sm.formats {
		stats := format.rtcpReceiver.Stats()
		if stats != nil && stats.RemoteSSRC == ssrc {
			return format
		}
	}
	return nil
}

func (sm *serverSessionMedia) decodeRTP(payload []byte) (*rtp.Packet, error) {
	if sm.srtpInCtx != nil {
		var err error
		payload, err = sm.srtpInCtx.decryptRTP(payload, payload, nil)
		if err != nil {
			return nil, err
		}
	}

	var pkt rtp.Packet
	err := pkt.Unmarshal(payload)
	return &pkt, err
}

func (sm *serverSessionMedia) decodeRTCP(payload []byte) ([]rtcp.Packet, error) {
	if sm.srtpInCtx != nil {
		var err error
		payload, err = sm.srtpInCtx.decryptRTCP(payload, payload, nil)
		if err != nil {
			return nil, err
		}
	}

	pkts, err := rtcp.Unmarshal(payload)
	if err != nil {
		return nil, err
	}

	return pkts, nil
}

func (sm *serverSessionMedia) readPacketRTPUDPPlay(payload []byte) bool {
	atomic.AddUint64(sm.bytesReceived, uint64(len(payload)))

	if len(payload) == (udpMaxPayloadSize + 1) {
		sm.onPacketRTPDecodeError(liberrors.ErrServerRTPPacketTooBigUDP{})
		return false
	}

	pkt, err := sm.decodeRTP(payload)
	if err != nil {
		sm.onPacketRTPDecodeError(err)
		return false
	}

	forma, ok := sm.formats[pkt.PayloadType]
	if !ok {
		sm.onPacketRTPDecodeError(liberrors.ErrServerRTPPacketUnknownPayloadType{PayloadType: pkt.PayloadType})
		return false
	}

	now := sm.ss.s.timeNow()

	forma.readPacketRTPUDP(pkt, now)

	return true
}

func (sm *serverSessionMedia) readPacketRTCPUDPPlay(payload []byte) bool {
	atomic.AddUint64(sm.bytesReceived, uint64(len(payload)))

	if len(payload) == (udpMaxPayloadSize + 1) {
		sm.onPacketRTCPDecodeError(liberrors.ErrServerRTCPPacketTooBigUDP{})
		return false
	}

	packets, err := sm.decodeRTCP(payload)
	if err != nil {
		sm.onPacketRTCPDecodeError(err)
		return false
	}

	now := sm.ss.s.timeNow()
	atomic.StoreInt64(sm.ss.udpLastPacketTime, now.Unix())

	atomic.AddUint64(sm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		sm.onPacketRTCP(pkt)
	}

	return true
}

func (sm *serverSessionMedia) readPacketRTPUDPRecord(payload []byte) bool {
	atomic.AddUint64(sm.bytesReceived, uint64(len(payload)))

	if len(payload) == (udpMaxPayloadSize + 1) {
		sm.onPacketRTPDecodeError(liberrors.ErrServerRTPPacketTooBigUDP{})
		return false
	}

	pkt, err := sm.decodeRTP(payload)
	if err != nil {
		sm.onPacketRTPDecodeError(err)
		return false
	}

	forma, ok := sm.formats[pkt.PayloadType]
	if !ok {
		sm.onPacketRTPDecodeError(liberrors.ErrServerRTPPacketUnknownPayloadType{PayloadType: pkt.PayloadType})
		return false
	}

	now := sm.ss.s.timeNow()
	atomic.StoreInt64(sm.ss.udpLastPacketTime, now.Unix())

	forma.readPacketRTPUDP(pkt, now)

	return true
}

func (sm *serverSessionMedia) readPacketRTCPUDPRecord(payload []byte) bool {
	atomic.AddUint64(sm.bytesReceived, uint64(len(payload)))

	if len(payload) == (udpMaxPayloadSize + 1) {
		sm.onPacketRTCPDecodeError(liberrors.ErrServerRTCPPacketTooBigUDP{})
		return false
	}

	packets, err := sm.decodeRTCP(payload)
	if err != nil {
		sm.onPacketRTCPDecodeError(err)
		return false
	}

	now := sm.ss.s.timeNow()
	atomic.StoreInt64(sm.ss.udpLastPacketTime, now.Unix())

	atomic.AddUint64(sm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		if sr, ok := pkt.(*rtcp.SenderReport); ok {
			format := sm.findFormatByRemoteSSRC(sr.SSRC)
			if format != nil {
				format.rtcpReceiver.ProcessSenderReport(sr, now)
			}
		}

		sm.onPacketRTCP(pkt)
	}

	return true
}

func (sm *serverSessionMedia) readPacketRTPTCPPlay(payload []byte) bool {
	if !sm.media.IsBackChannel {
		return false
	}

	atomic.AddUint64(sm.bytesReceived, uint64(len(payload)))

	pkt, err := sm.decodeRTP(payload)
	if err != nil {
		sm.onPacketRTPDecodeError(err)
		return false
	}

	forma, ok := sm.formats[pkt.PayloadType]
	if !ok {
		sm.onPacketRTPDecodeError(liberrors.ErrServerRTPPacketUnknownPayloadType{PayloadType: pkt.PayloadType})
		return false
	}

	forma.readPacketRTPTCP(pkt)

	return true
}

func (sm *serverSessionMedia) readPacketRTCPTCPPlay(payload []byte) bool {
	atomic.AddUint64(sm.bytesReceived, uint64(len(payload)))

	if len(payload) > udpMaxPayloadSize {
		sm.onPacketRTCPDecodeError(liberrors.ErrServerRTCPPacketTooBig{L: len(payload), Max: udpMaxPayloadSize})
		return false
	}

	packets, err := sm.decodeRTCP(payload)
	if err != nil {
		sm.onPacketRTCPDecodeError(err)
		return false
	}

	atomic.AddUint64(sm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		sm.onPacketRTCP(pkt)
	}

	return true
}

func (sm *serverSessionMedia) readPacketRTPTCPRecord(payload []byte) bool {
	atomic.AddUint64(sm.bytesReceived, uint64(len(payload)))

	pkt, err := sm.decodeRTP(payload)
	if err != nil {
		sm.onPacketRTPDecodeError(err)
		return false
	}

	forma, ok := sm.formats[pkt.PayloadType]
	if !ok {
		sm.onPacketRTPDecodeError(liberrors.ErrServerRTPPacketUnknownPayloadType{PayloadType: pkt.PayloadType})
		return false
	}

	forma.readPacketRTPTCP(pkt)

	return true
}

func (sm *serverSessionMedia) readPacketRTCPTCPRecord(payload []byte) bool {
	atomic.AddUint64(sm.bytesReceived, uint64(len(payload)))

	if len(payload) > udpMaxPayloadSize {
		sm.onPacketRTCPDecodeError(liberrors.ErrServerRTCPPacketTooBig{L: len(payload), Max: udpMaxPayloadSize})
		return false
	}

	packets, err := sm.decodeRTCP(payload)
	if err != nil {
		sm.onPacketRTCPDecodeError(err)
		return false
	}

	now := sm.ss.s.timeNow()

	atomic.AddUint64(sm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		if sr, ok := pkt.(*rtcp.SenderReport); ok {
			format := sm.findFormatByRemoteSSRC(sr.SSRC)
			if format != nil {
				format.rtcpReceiver.ProcessSenderReport(sr, now)
			}
		}

		sm.onPacketRTCP(pkt)
	}

	return true
}

func (sm *serverSessionMedia) onPacketRTPDecodeError(err error) {
	atomic.AddUint64(sm.rtpPacketsInError, 1)

	if h, ok := sm.ss.s.Handler.(ServerHandlerOnDecodeError); ok {
		h.OnDecodeError(&ServerHandlerOnDecodeErrorCtx{
			Session: sm.ss,
			Error:   err,
		})
	} else {
		log.Println(err.Error())
	}
}

func (sm *serverSessionMedia) onPacketRTCPDecodeError(err error) {
	atomic.AddUint64(sm.rtcpPacketsInError, 1)

	if h, ok := sm.ss.s.Handler.(ServerHandlerOnDecodeError); ok {
		h.OnDecodeError(&ServerHandlerOnDecodeErrorCtx{
			Session: sm.ss,
			Error:   err,
		})
	} else {
		log.Println(err.Error())
	}
}

func (sm *serverSessionMedia) writePacketRTCP(pkt rtcp.Packet) error {
	plain, err := pkt.Marshal()
	if err != nil {
		return err
	}

	maxPlainPacketSize := sm.ss.s.MaxPacketSize
	if sm.ss.setuppedSecure {
		maxPlainPacketSize -= srtcpOverhead
	}

	if len(plain) > maxPlainPacketSize {
		return fmt.Errorf("packet is too big")
	}

	var encr []byte
	if sm.ss.setuppedSecure {
		encr = make([]byte, sm.ss.s.MaxPacketSize)
		encr, err = sm.srtpOutCtx.encryptRTCP(encr, plain, nil)
		if err != nil {
			return err
		}
	}

	if sm.ss.setuppedSecure {
		return sm.writePacketRTCPEncoded(encr)
	}
	return sm.writePacketRTCPEncoded(plain)
}

func (sm *serverSessionMedia) writePacketRTCPEncoded(payload []byte) error {
	sm.ss.writerMutex.RLock()
	defer sm.ss.writerMutex.RUnlock()

	if sm.ss.writer == nil {
		return nil
	}

	ok := sm.ss.writer.push(func() error {
		return sm.writePacketRTCPInQueue(payload)
	})
	if !ok {
		return liberrors.ErrServerWriteQueueFull{}
	}

	return nil
}

func (sm *serverSessionMedia) writePacketRTCPInQueueUDP(payload []byte) error {
	err := sm.ss.s.udpRTCPListener.write(payload, sm.udpRTCPWriteAddr)
	if err != nil {
		return err
	}

	atomic.AddUint64(sm.bytesSent, uint64(len(payload)))
	atomic.AddUint64(sm.rtcpPacketsSent, 1)
	return nil
}

func (sm *serverSessionMedia) writePacketRTCPInQueueTCP(payload []byte) error {
	sm.ss.tcpFrame.Channel = sm.tcpChannel + 1
	sm.ss.tcpFrame.Payload = payload
	sm.ss.tcpConn.nconn.SetWriteDeadline(time.Now().Add(sm.ss.s.WriteTimeout))
	err := sm.ss.tcpConn.conn.WriteInterleavedFrame(sm.ss.tcpFrame, sm.ss.tcpBuffer)
	if err != nil {
		return err
	}

	atomic.AddUint64(sm.bytesSent, uint64(len(payload)))
	atomic.AddUint64(sm.rtcpPacketsSent, 1)
	return nil
}
