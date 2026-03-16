package gortsplib

import (
	"fmt"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
)

type serverSessionMedia struct {
	ss               *ServerSession
	media            *description.Media
	localSSRCs       map[uint8]uint32
	srtpInCtx        *wrappedSRTPContext
	srtpOutCtx       *wrappedSRTPContext
	udpRTPReadPort   int
	udpRTPWriteAddr  *net.UDPAddr
	udpRTCPReadPort  int
	udpRTCPWriteAddr *net.UDPAddr
	tcpChannel       int
	onPacketRTCP     OnPacketRTCPFunc

	formats                map[uint8]*serverSessionFormat // record only
	writePacketRTCPInQueue func([]byte) error
	bytesReceived          *uint64
	bytesSent              *uint64
	rtpPacketsInError      *uint64
	rtcpPacketsReceived    *uint64
	rtcpPacketsSent        *uint64
	rtcpPacketsInError     *uint64
}

func (ssm *serverSessionMedia) initialize() {
	ssm.bytesReceived = new(uint64)
	ssm.bytesSent = new(uint64)
	ssm.rtpPacketsInError = new(uint64)
	ssm.rtcpPacketsReceived = new(uint64)
	ssm.rtcpPacketsSent = new(uint64)
	ssm.rtcpPacketsInError = new(uint64)

	ssm.formats = make(map[uint8]*serverSessionFormat)

	for _, forma := range ssm.media.Formats {
		f := &serverSessionFormat{
			ssm:         ssm,
			format:      forma,
			localSSRC:   ssm.localSSRCs[forma.PayloadType()],
			onPacketRTP: func(*rtp.Packet) {},
		}
		f.initialize()
		ssm.formats[forma.PayloadType()] = f
	}

	switch ssm.ss.setuppedTransport.Protocol {
	case ProtocolUDP, ProtocolUDPMulticast:
		ssm.writePacketRTCPInQueue = ssm.writePacketRTCPInQueueUDP

	case ProtocolTCP:
		ssm.writePacketRTCPInQueue = ssm.writePacketRTCPInQueueTCP

		if ssm.ss.tcpCallbackByChannel == nil {
			ssm.ss.tcpCallbackByChannel = make(map[int]readFunc)
		}

		if ssm.ss.state == ServerSessionStateInitial || ssm.ss.state == ServerSessionStatePrePlay {
			ssm.ss.tcpCallbackByChannel[ssm.tcpChannel] = ssm.readPacketRTPTCPPlay
			ssm.ss.tcpCallbackByChannel[ssm.tcpChannel+1] = ssm.readPacketRTCPTCPPlay
		} else {
			ssm.ss.tcpCallbackByChannel[ssm.tcpChannel] = ssm.readPacketRTPTCPRecord
			ssm.ss.tcpCallbackByChannel[ssm.tcpChannel+1] = ssm.readPacketRTCPTCPRecord
		}
	}
}

func (ssm *serverSessionMedia) close() {
	ssm.stop()

	for _, forma := range ssm.formats {
		forma.close()
	}
}

func (ssm *serverSessionMedia) start() error {
	switch ssm.ss.setuppedTransport.Protocol {
	case ProtocolUDP, ProtocolUDPMulticast:
		if ssm.ss.setuppedTransport.Protocol == ProtocolUDP {
			if ssm.ss.state == ServerSessionStatePlay {
				if ssm.media.IsBackChannel {
					ssm.ss.s.udpRTPListener.addClient(ssm.ss.author.ip(), ssm.udpRTPReadPort, ssm.readPacketRTPUDPPlay)
				}
				ssm.ss.s.udpRTCPListener.addClient(ssm.ss.author.ip(), ssm.udpRTCPReadPort, ssm.readPacketRTCPUDPPlay)
			} else {
				// open the firewall by sending empty packets to the remote part.
				buf, _ := (&rtp.Packet{Header: rtp.Header{Version: 2}}).Marshal()
				if ssm.srtpOutCtx != nil {
					encr := make([]byte, ssm.ss.s.MaxPacketSize)
					encr, err := ssm.srtpOutCtx.encryptRTP(encr, buf, nil)
					if err != nil {
						return err
					}
					buf = encr
				}
				err := ssm.ss.s.udpRTPListener.write(buf, ssm.udpRTPWriteAddr)
				if err != nil {
					return err
				}

				buf, _ = (&rtcp.ReceiverReport{}).Marshal()
				if ssm.srtpOutCtx != nil {
					encr := make([]byte, ssm.ss.s.MaxPacketSize)
					encr, err = ssm.srtpOutCtx.encryptRTCP(encr, buf, nil)
					if err != nil {
						return err
					}
					buf = encr
				}
				err = ssm.ss.s.udpRTCPListener.write(buf, ssm.udpRTCPWriteAddr)
				if err != nil {
					return err
				}

				ssm.ss.s.udpRTPListener.addClient(ssm.ss.author.ip(), ssm.udpRTPReadPort, ssm.readPacketRTPUDPRecord)
				ssm.ss.s.udpRTCPListener.addClient(ssm.ss.author.ip(), ssm.udpRTCPReadPort, ssm.readPacketRTCPUDPRecord)
			}
		}
	}

	return nil
}

func (ssm *serverSessionMedia) stop() {
	if ssm.ss.setuppedTransport.Protocol == ProtocolUDP {
		ssm.ss.s.udpRTPListener.removeClient(ssm.ss.author.ip(), ssm.udpRTPReadPort)
		ssm.ss.s.udpRTCPListener.removeClient(ssm.ss.author.ip(), ssm.udpRTCPReadPort)
	}
}

func (ssm *serverSessionMedia) stats() SessionStatsMedia { //nolint:dupl
	return SessionStatsMedia{
		InboundBytes:              atomic.LoadUint64(ssm.bytesReceived),
		InboundRTPPacketsInError:  atomic.LoadUint64(ssm.rtpPacketsInError),
		InboundRTCPPackets:        atomic.LoadUint64(ssm.rtcpPacketsReceived),
		InboundRTCPPacketsInError: atomic.LoadUint64(ssm.rtcpPacketsInError),
		OutboundBytes:             atomic.LoadUint64(ssm.bytesSent),
		OutboundRTCPPackets:       atomic.LoadUint64(ssm.rtcpPacketsSent),
		Formats: func() map[format.Format]SessionStatsFormat {
			ret := make(map[format.Format]SessionStatsFormat, len(ssm.formats))
			for _, ssf := range ssm.formats {
				ret[ssf.format] = ssf.stats()
			}
			return ret
		}(),
		// deprecated
		BytesReceived:       atomic.LoadUint64(ssm.bytesReceived),
		BytesSent:           atomic.LoadUint64(ssm.bytesSent),
		RTPPacketsInError:   atomic.LoadUint64(ssm.rtpPacketsInError),
		RTCPPacketsReceived: atomic.LoadUint64(ssm.rtcpPacketsReceived),
		RTCPPacketsSent:     atomic.LoadUint64(ssm.rtcpPacketsSent),
		RTCPPacketsInError:  atomic.LoadUint64(ssm.rtcpPacketsInError),
	}
}

func (ssm *serverSessionMedia) findFormatByRemoteSSRC(ssrc uint32) *serverSessionFormat {
	for _, sf := range ssm.formats {
		if v, ok := sf.remoteSSRC(); ok && v == ssrc {
			return sf
		}
	}
	return nil
}

func (ssm *serverSessionMedia) decodeRTP(payload []byte, header *rtp.Header, headerSize int) (*rtp.Packet, error) {
	if ssm.srtpInCtx != nil {
		var err error
		payload, err = ssm.srtpInCtx.decryptRTP(payload, payload, header)
		if err != nil {
			return nil, err
		}
	}

	return fastRTPUnmarshal(payload, header, headerSize)
}

func (ssm *serverSessionMedia) decodeRTCP(payload []byte) ([]rtcp.Packet, error) {
	if ssm.srtpInCtx != nil {
		var err error
		payload, err = ssm.srtpInCtx.decryptRTCP(payload, payload, nil)
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

func (ssm *serverSessionMedia) readPacketRTP(payload []byte, now time.Time) bool {
	var header rtp.Header
	headerSize, err := header.Unmarshal(payload)
	if err != nil {
		ssm.onPacketRTPDecodeError(err)
		return false
	}

	forma, ok := ssm.formats[header.PayloadType]
	if !ok {
		ssm.onPacketRTPDecodeError(liberrors.ErrServerRTPPacketUnknownPayloadType{PayloadType: header.PayloadType})
		return false
	}

	return forma.readPacketRTP(payload, &header, headerSize, now)
}

func (ssm *serverSessionMedia) readPacketRTCPPlay(payload []byte) bool {
	packets, err := ssm.decodeRTCP(payload)
	if err != nil {
		ssm.onPacketRTCPDecodeError(err)
		return false
	}

	atomic.AddUint64(ssm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		ssm.onPacketRTCP(pkt)
	}

	return true
}

func (ssm *serverSessionMedia) readPacketRTCPRecord(payload []byte) bool {
	packets, err := ssm.decodeRTCP(payload)
	if err != nil {
		ssm.onPacketRTCPDecodeError(err)
		return false
	}

	now := ssm.ss.s.timeNow()

	atomic.AddUint64(ssm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		if sr, ok := pkt.(*rtcp.SenderReport); ok {
			format := ssm.findFormatByRemoteSSRC(sr.SSRC)
			if format != nil {
				format.rtpReceiver.ProcessSenderReport(sr, now)
			}
		}

		ssm.onPacketRTCP(pkt)
	}

	return true
}

func (ssm *serverSessionMedia) readPacketRTPUDPPlay(payload []byte) bool {
	atomic.AddUint64(ssm.bytesReceived, uint64(len(payload)))

	now := ssm.ss.s.timeNow()
	atomic.StoreInt64(ssm.ss.udpLastPacketTime, now.Unix())

	if len(payload) == (udpMaxPayloadSize + 1) {
		ssm.onPacketRTPDecodeError(liberrors.ErrServerRTPPacketTooBigUDP{})
		return false
	}

	return ssm.readPacketRTP(payload, ssm.ss.s.timeNow())
}

func (ssm *serverSessionMedia) readPacketRTCPUDPPlay(payload []byte) bool {
	atomic.AddUint64(ssm.bytesReceived, uint64(len(payload)))

	now := ssm.ss.s.timeNow()
	atomic.StoreInt64(ssm.ss.udpLastPacketTime, now.Unix())

	if len(payload) == (udpMaxPayloadSize + 1) {
		ssm.onPacketRTCPDecodeError(liberrors.ErrServerRTCPPacketTooBigUDP{})
		return false
	}

	return ssm.readPacketRTCPPlay(payload)
}

func (ssm *serverSessionMedia) readPacketRTPUDPRecord(payload []byte) bool {
	atomic.AddUint64(ssm.bytesReceived, uint64(len(payload)))

	now := ssm.ss.s.timeNow()
	atomic.StoreInt64(ssm.ss.udpLastPacketTime, now.Unix())

	if len(payload) == (udpMaxPayloadSize + 1) {
		ssm.onPacketRTPDecodeError(liberrors.ErrServerRTPPacketTooBigUDP{})
		return false
	}

	return ssm.readPacketRTP(payload, ssm.ss.s.timeNow())
}

func (ssm *serverSessionMedia) readPacketRTCPUDPRecord(payload []byte) bool {
	atomic.AddUint64(ssm.bytesReceived, uint64(len(payload)))

	now := ssm.ss.s.timeNow()
	atomic.StoreInt64(ssm.ss.udpLastPacketTime, now.Unix())

	if len(payload) == (udpMaxPayloadSize + 1) {
		ssm.onPacketRTCPDecodeError(liberrors.ErrServerRTCPPacketTooBigUDP{})
		return false
	}

	return ssm.readPacketRTCPRecord(payload)
}

func (ssm *serverSessionMedia) readPacketRTPTCPPlay(payload []byte) bool {
	if !ssm.media.IsBackChannel {
		return false
	}

	atomic.AddUint64(ssm.bytesReceived, uint64(len(payload)))

	return ssm.readPacketRTP(payload, ssm.ss.s.timeNow())
}

func (ssm *serverSessionMedia) readPacketRTCPTCPPlay(payload []byte) bool {
	atomic.AddUint64(ssm.bytesReceived, uint64(len(payload)))

	if len(payload) > udpMaxPayloadSize {
		ssm.onPacketRTCPDecodeError(liberrors.ErrServerRTCPPacketTooBig{L: len(payload), Max: udpMaxPayloadSize})
		return false
	}

	return ssm.readPacketRTCPPlay(payload)
}

func (ssm *serverSessionMedia) readPacketRTPTCPRecord(payload []byte) bool {
	atomic.AddUint64(ssm.bytesReceived, uint64(len(payload)))

	return ssm.readPacketRTP(payload, ssm.ss.s.timeNow())
}

func (ssm *serverSessionMedia) readPacketRTCPTCPRecord(payload []byte) bool {
	atomic.AddUint64(ssm.bytesReceived, uint64(len(payload)))

	if len(payload) > udpMaxPayloadSize {
		ssm.onPacketRTCPDecodeError(liberrors.ErrServerRTCPPacketTooBig{L: len(payload), Max: udpMaxPayloadSize})
		return false
	}

	return ssm.readPacketRTCPRecord(payload)
}

func (ssm *serverSessionMedia) onPacketRTPDecodeError(err error) {
	atomic.AddUint64(ssm.rtpPacketsInError, 1)

	if h, ok := ssm.ss.s.Handler.(ServerHandlerOnDecodeError); ok {
		h.OnDecodeError(&ServerHandlerOnDecodeErrorCtx{
			Session: ssm.ss,
			Error:   err,
		})
	} else {
		log.Println(err.Error())
	}
}

func (ssm *serverSessionMedia) onPacketRTCPDecodeError(err error) {
	atomic.AddUint64(ssm.rtcpPacketsInError, 1)

	if h, ok := ssm.ss.s.Handler.(ServerHandlerOnDecodeError); ok {
		h.OnDecodeError(&ServerHandlerOnDecodeErrorCtx{
			Session: ssm.ss,
			Error:   err,
		})
	} else {
		log.Println(err.Error())
	}
}

func (ssm *serverSessionMedia) writePacketRTCP(pkt rtcp.Packet) error {
	plain, err := pkt.Marshal()
	if err != nil {
		return err
	}

	maxPlainPacketSize := ssm.ss.s.MaxPacketSize
	if ssm.srtpOutCtx != nil {
		maxPlainPacketSize -= srtcpOverhead
	}

	if len(plain) > maxPlainPacketSize {
		return fmt.Errorf("packet is too big")
	}

	var encr []byte
	if ssm.srtpOutCtx != nil {
		encr = make([]byte, ssm.ss.s.MaxPacketSize)
		encr, err = ssm.srtpOutCtx.encryptRTCP(encr, plain, nil)
		if err != nil {
			return err
		}
	}

	if ssm.srtpOutCtx != nil {
		return ssm.writePacketRTCPEncoded(encr)
	}
	return ssm.writePacketRTCPEncoded(plain)
}

func (ssm *serverSessionMedia) writePacketRTCPEncoded(payload []byte) error {
	ssm.ss.writerMutex.RLock()
	defer ssm.ss.writerMutex.RUnlock()

	if ssm.ss.writer == nil {
		return nil
	}

	ok := ssm.ss.writer.Push(func() error {
		return ssm.writePacketRTCPInQueue(payload)
	})
	if !ok {
		return liberrors.ErrServerWriteQueueFull{}
	}

	return nil
}

func (ssm *serverSessionMedia) writePacketRTCPInQueueUDP(payload []byte) error {
	err := ssm.ss.s.udpRTCPListener.write(payload, ssm.udpRTCPWriteAddr)
	if err != nil {
		return err
	}

	atomic.AddUint64(ssm.bytesSent, uint64(len(payload)))
	atomic.AddUint64(ssm.rtcpPacketsSent, 1)
	return nil
}

func (ssm *serverSessionMedia) writePacketRTCPInQueueTCP(payload []byte) error {
	ssm.ss.tcpFrame.Channel = ssm.tcpChannel + 1
	ssm.ss.tcpFrame.Payload = payload
	ssm.ss.tcpConn.nconn.SetWriteDeadline(time.Now().Add(ssm.ss.s.WriteTimeout))
	err := ssm.ss.tcpConn.conn.WriteInterleavedFrame(ssm.ss.tcpFrame, ssm.ss.tcpBuffer)
	if err != nil {
		return err
	}

	atomic.AddUint64(ssm.bytesSent, uint64(len(payload)))
	atomic.AddUint64(ssm.rtcpPacketsSent, 1)
	return nil
}
