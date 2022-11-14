package rtpvp9

import (
	"bytes"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func mergeBytes(vals ...[]byte) []byte {
	size := 0
	for _, v := range vals {
		size += len(v)
	}
	res := make([]byte, size)

	pos := 0
	for _, v := range vals {
		n := copy(res[pos:], v)
		pos += n
	}

	return res
}

var cases = []struct {
	name  string
	frame []byte
	pts   time.Duration
	pkts  []*rtp.Packet
}{
	{
		"single",
		[]byte{0x01, 0x02, 0x03, 0x04},
		25 * time.Millisecond,
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289528607,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x9c, 0xb5, 0xaf, 0x01, 0x02, 0x03, 0x04},
			},
		},
	},
	{
		"fragmented",
		bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 4096/4),
		55 * time.Millisecond,
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes([]byte{0x98, 0xb5, 0xaf}, bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 364), []byte{0x01}),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17646,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes([]byte{0x90, 0xb5, 0xaf, 0x02, 0x03, 0x04},
					bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 363), []byte{0x01, 0x02}),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17647,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes([]byte{0x94, 0xb5, 0xaf, 0x03, 0x04}, bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 295)),
			},
		},
	},
}

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			d.Init()

			// send an initial packet downstream
			// in order to compute the right timestamp,
			// that is relative to the initial packet
			pkt := rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x9c, 0xb5, 0xaf, 0x01, 0x02, 0x03, 0x04},
			}
			_, _, err := d.Decode(&pkt)
			require.NoError(t, err)

			var frame []byte

			for _, pkt := range ca.pkts {
				var pts time.Duration
				frame, pts, err = d.Decode(pkt)
				if err == ErrMorePacketsNeeded {
					continue
				}

				require.NoError(t, err)
				require.Equal(t, ca.pts, pts)
			}

			require.Equal(t, ca.frame, frame)
		})
	}
}
