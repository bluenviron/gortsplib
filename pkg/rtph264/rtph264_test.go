package rtph264

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/pion/rtp/v2"
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
	nalus [][]byte
	pts   time.Duration
	pkts  []*rtp.Packet
}{
	{
		"single",
		[][]byte{
			mergeBytes(
				[]byte{0x05},
				bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 8),
			),
		},
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
				Payload: mergeBytes(
					[]byte{0x05},
					bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 8),
				),
			},
		},
	},
	{
		"negative timestamp",
		[][]byte{
			mergeBytes(
				[]byte{0x05},
				bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 8),
			),
		},
		-20 * time.Millisecond,
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289524557,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x05},
					bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 8),
				),
			},
		},
	},
	{
		"fragmented",
		[][]byte{
			mergeBytes(
				[]byte{0x05},
				bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 512),
			),
		},
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
				Payload: mergeBytes(
					[]byte{0x1c, 0x85},
					bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
					[]byte{0x00, 0x01},
				),
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
				Payload: mergeBytes(
					[]byte{0x1c, 0x05},
					[]byte{0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
					bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 181),
					[]byte{0x00, 0x01, 0x02, 0x03},
				),
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
				Payload: mergeBytes(
					[]byte{0x1c, 0x45},
					[]byte{0x04, 0x05, 0x06, 0x07},
					bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 147),
				),
			},
		},
	},
	{
		"aggregated",
		[][]byte{
			{0x09, 0xF0},
			{
				0x41, 0x9a, 0x24, 0x6c, 0x41, 0x4f, 0xfe, 0xd6,
				0x8c, 0xb0, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
				0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
				0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
				0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
				0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
				0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
				0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
				0x00, 0x00, 0x6d, 0x40,
			},
		},
		0,
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{
					0x18, 0x00, 0x02, 0x09,
					0xf0, 0x00, 0x44, 0x41, 0x9a, 0x24, 0x6c, 0x41,
					0x4f, 0xfe, 0xd6, 0x8c, 0xb0, 0x00, 0x00, 0x03,
					0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
					0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
					0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
					0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
					0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
					0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
					0x00, 0x00, 0x03, 0x00, 0x00, 0x6d, 0x40,
				},
			},
		},
	},
	{
		"aggregated followed by single",
		[][]byte{
			{0x09, 0xF0},
			{
				0x41, 0x9a, 0x24, 0x6c, 0x41, 0x4f, 0xfe, 0xd6,
				0x8c, 0xb0, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
				0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
				0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
				0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
				0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
				0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
				0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
				0x00, 0x00, 0x6d, 0x40,
			},
			mergeBytes(
				[]byte{0x08},
				bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 175),
			),
		},
		0,
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{
					0x18, 0x00, 0x02, 0x09,
					0xf0, 0x00, 0x44, 0x41, 0x9a, 0x24, 0x6c, 0x41,
					0x4f, 0xfe, 0xd6, 0x8c, 0xb0, 0x00, 0x00, 0x03,
					0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
					0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
					0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
					0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
					0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
					0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
					0x00, 0x00, 0x03, 0x00, 0x00, 0x6d, 0x40,
				},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17646,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x08},
					bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 175),
				),
			},
		},
	},
	{
		"fragmented followed by aggregated",
		[][]byte{
			mergeBytes(
				[]byte{0x05},
				bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 256),
			),
			{0x09, 0xF0},
			{0x09, 0xF0},
		},
		0,
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x1c, 0x85},
					bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
					[]byte{0x00, 0x01},
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17646,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x1c, 0x45},
					[]byte{0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
					bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 73),
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17647,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{
					0x18, 0x00, 0x02, 0x09,
					0xf0, 0x00, 0x02, 0x09, 0xf0,
				},
			},
		},
	},
}

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := NewDecoder()

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
				Payload: []byte{0x06, 0x00},
			}
			_, _, err := d.Decode(&pkt)
			require.NoError(t, err)

			var nalus [][]byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				addNALUs, pts, err := d.Decode(pkt)
				if err == ErrMorePacketsNeeded {
					continue
				}

				require.NoError(t, err)
				require.Equal(t, ca.pts, pts)
				nalus = append(nalus, addNALUs...)

				// test input integrity
				require.Equal(t, clone, pkt)
			}

			require.Equal(t, ca.nalus, nalus)
		})
	}
}

func TestDecodePartOfFragmentedBeforeSingle(t *testing.T) {
	d := NewDecoder()

	pkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17647,
			Timestamp:      2289531307,
			SSRC:           0x9dbb7812,
		},
		Payload: mergeBytes(
			[]byte{0x1c, 0x45},
			[]byte{0x04, 0x05, 0x06, 0x07},
			bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 147),
		),
	}
	_, _, err := d.Decode(&pkt)
	require.Equal(t, ErrNonStartingPacketAndNoPrevious, err)

	pkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17645,
			Timestamp:      2289528607,
			SSRC:           0x9dbb7812,
		},
		Payload: mergeBytes(
			[]byte{0x05},
			bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 8),
		),
	}
	_, _, err = d.Decode(&pkt)
	require.NoError(t, err)
}

func TestDecodeSTAPAWithPadding(t *testing.T) {
	pkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17645,
			Timestamp:      2289526357,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{
			0x18, 0x00, 0x02, 0xaa,
			0xbb, 0x00, 0x02, 0xcc, 0xdd, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}
	nalus, _, err := NewDecoder().Decode(&pkt)
	require.NoError(t, err)
	require.Equal(t, [][]byte{
		{0xaa, 0xbb},
		{0xcc, 0xdd},
	}, nalus)
}

func TestDecodeErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		pkts []*rtp.Packet
		err  string
	}{
		{
			"missing payload",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
				},
			},
			"payload is too short",
		},
		{
			"STAP-A without NALUs",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{0x18},
				},
			},
			"STAP-A packet doesn't contain any NALU",
		},
		{
			"STAP-A without size",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{0x18, 0x01},
				},
			},
			"invalid STAP-A packet (invalid size)",
		},
		{
			"STAP-A with invalid size",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{0x18, 0x00, 0x15},
				},
			},
			"invalid STAP-A packet (invalid size)",
		},
		{
			"FU-A without payload",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{0x1c},
				},
			},
			"invalid FU-A packet (invalid size)",
		},
		{
			"FU-A without start bit",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: mergeBytes(
						[]byte{0x05},
						bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 8),
					),
				},
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17646,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{0x1c, 0x00},
				},
			},
			"invalid FU-A packet (non-starting)",
		},
		{
			"FU-A with 2nd packet empty",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         false,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: mergeBytes(
						[]byte{0x1c, 0x85},
						bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
						[]byte{0x00, 0x01},
					),
				},
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         false,
						PayloadType:    96,
						SequenceNumber: 17646,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
				},
			},
			"invalid FU-A packet (invalid size)",
		},
		{
			"FU-A with 2nd packet invalid",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         false,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: mergeBytes(
						[]byte{
							0x1c, 0x85,
						},
						bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
						[]byte{0x00, 0x01},
					),
				},
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         false,
						PayloadType:    96,
						SequenceNumber: 17646,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{0x01, 0x00},
				},
			},
			"expected FU-A packet, got another type",
		},
		{
			"FU-A with two starting packets",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         false,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: mergeBytes(
						[]byte{0x1c, 0x85},
						bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
						[]byte{0x00, 0x01},
					),
				},
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         false,
						PayloadType:    96,
						SequenceNumber: 17646,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: mergeBytes(
						[]byte{0x1c, 0x85},
						bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
						[]byte{0x00, 0x01},
					),
				},
			},
			"invalid FU-A packet (decoded two starting packets in a row)",
		},
		{
			"MTAP",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         false,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{0x1a},
				},
			},
			"packet type not supported (MTAP-16)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			d := NewDecoder()
			var lastErr error
			for _, pkt := range ca.pkts {
				_, _, lastErr = d.Decode(pkt)
			}
			require.EqualError(t, lastErr, ca.err)
		})
	}
}

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			sequenceNumber := uint16(0x44ed)
			ssrc := uint32(0x9dbb7812)
			initialTs := uint32(0x88776655)
			e := NewEncoder(96, &sequenceNumber, &ssrc, &initialTs)

			pkts, err := e.Encode(ca.nalus, ca.pts)
			require.NoError(t, err)
			require.Equal(t, ca.pkts, pkts)
		})
	}
}

func TestEncodeRandomInitialState(t *testing.T) {
	NewEncoder(96, nil, nil, nil)
}

type dummyReader struct {
	byts [][]byte
	i    int
}

func (f *dummyReader) Read(p []byte) (int, error) {
	if f.i >= len(f.byts) {
		return 0, io.EOF
	}
	n := copy(p, f.byts[f.i])
	f.i++
	return n, nil
}

func TestReadSPSPPS(t *testing.T) {
	for _, ca := range []struct {
		name string
		byts [][]byte
		sps  []byte
		pps  []byte
	}{
		{
			"sps then pps",
			[][]byte{
				{128, 96, 61, 205, 54, 67, 90, 125, 40, 249, 97, 176, 7, 1, 2},
				{128, 96, 61, 206, 54, 67, 90, 125, 40, 249, 97, 176, 8, 3, 4},
			},
			[]byte{0x07, 0x01, 0x02},
			[]byte{0x08, 0x03, 0x04},
		},
		{
			"pps then sps",
			[][]byte{
				{128, 96, 61, 206, 54, 67, 90, 125, 40, 249, 97, 176, 8, 3, 4},
				{128, 96, 61, 205, 54, 67, 90, 125, 40, 249, 97, 176, 7, 1, 2},
			},
			[]byte{0x07, 0x01, 0x02},
			[]byte{0x08, 0x03, 0x04},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			sps, pps, err := NewDecoder().ReadSPSPPS(&dummyReader{byts: ca.byts})
			require.NoError(t, err)
			require.Equal(t, ca.sps, sps)
			require.Equal(t, ca.pps, pps)
		})
	}
}

func TestReadSPSPPSErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		byts [][]byte
		err  string
	}{
		{
			"empty",
			[][]byte{},
			"EOF",
		},
		{
			"more packets needed, then empty",
			[][]byte{
				mergeBytes(
					[]byte{
						0x80, 0x60, 0x44, 0xed, 0x88, 0x77, 0x79, 0xab,
						0x9d, 0xbb, 0x78, 0x12, 0x1c, 0x85,
					},
					bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
					[]byte{0x00, 0x01},
				),
			},
			"EOF",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, _, err := NewDecoder().ReadSPSPPS(&dummyReader{byts: ca.byts})
			require.EqualError(t, err, ca.err)
		})
	}
}
