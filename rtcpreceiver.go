package gortsplib

import (
	"sync"
	"time"
	"fmt"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/report"
	"github.com/pion/rtcp"
	"github.com/pion/rtp/v2"
)

type rtpReceiverTrack struct {
	chain *interceptor.Chain
	packetRTP  *rtp.Packet
	readerRTP  interceptor.RTPReader
	packetRTCP rtcp.Packet
	readerRTCP interceptor.RTCPReader
}

func newRTPReceiverTrack(ssrc uint32) *rtpReceiverTrack {
	track := &rtpReceiverTrack{}

	factory, _ := report.NewReceiverInterceptor(
		report.ReceiverInterval(1 * time.Second), // period
	)
	istance, _ := factory.NewInterceptor("")

	track.chain = interceptor.NewChain([]interceptor.Interceptor{
		istance,
	})

	track.readerRTP = track.chain.BindRemoteStream(
		&interceptor.StreamInfo{
			SSRC: ssrc,
		},
		interceptor.RTPReaderFunc(func([]byte, interceptor.Attributes) (int, interceptor.Attributes, error) {
			attrs := interceptor.Attributes{}
			attrs.Set(0, track.packetRTP.Header)
			return 0, attrs, nil
		}))

	track.readerRTCP = track.chain.BindRTCPReader(interceptor.RTCPReaderFunc(func(b []byte, attrs interceptor.Attributes) (int, interceptor.Attributes, error) {
		byts, _ := track.packetRTCP.Marshal()
		n := copy(b, byts)

		//attrs := interceptor.Attributes{}
		//attrs.Set(1, []rtcp.Packet{track.packetRTCP})
		fmt.Println("INCOMING RTCP", len(b), n, len(byts))
		return n, nil, nil
	}))

	track.chain.BindRTCPWriter(interceptor.RTCPWriterFunc(func(pkts []rtcp.Packet, attributes interceptor.Attributes) (int, error) {
		fmt.Println("TODO", pkts)
		return 0, nil
	}))

	return track
}

func (track *rtpReceiverTrack) close() {
	track.chain.Close()
}

type rtcpReceiver struct {
	period    time.Duration

	tracks    []*rtpReceiverTrack
	mutex     sync.Mutex
}

func newRTCPReceiver(period time.Duration, tracksLen int) *rtcpReceiver {
	rr := &rtcpReceiver{
		period: period,
		tracks: make([]*rtpReceiverTrack, tracksLen),
	}

	return rr
}

func (rr *rtcpReceiver) close() {
	for _, track := range rr.tracks {
		if track != nil {
			track.close()
		}
	}
}

func (rr *rtcpReceiver) processPacketRTP(now time.Time, trackID int, pkt *rtp.Packet) {
	track := func() *rtpReceiverTrack {
		rr.mutex.Lock()
		defer rr.mutex.Unlock()

		track := rr.tracks[trackID]
		if track == nil {
			track = newRTPReceiverTrack(pkt.SSRC)
			rr.tracks[trackID] = track
		}

		return track
	}()

	track.packetRTP = pkt
	track.readerRTP.Read(nil, nil)
}

func (rr *rtcpReceiver) processPacketRTCP(now time.Time, trackID int, pkt rtcp.Packet) {
	if sr, ok := (pkt).(*rtcp.SenderReport); ok {
		track := func() *rtpReceiverTrack {
			rr.mutex.Lock()
			defer rr.mutex.Unlock()

			track := rr.tracks[trackID]
			if track == nil {
				track = newRTPReceiverTrack(sr.SSRC)
				rr.tracks[trackID] = track
			}

			return track
		}()

		track.packetRTCP = pkt
		track.readerRTCP.Read(nil, nil)
	}
}
