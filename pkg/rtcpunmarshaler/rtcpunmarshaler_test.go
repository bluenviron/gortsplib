package rtcpunmarshaler

import (
	"testing"

	"github.com/pion/rtcp"
	"github.com/stretchr/testify/require"
)

var cases = []struct {
	name  string
	enc   []byte
	dec   []rtcp.Packet
	error string
}{
	{
		name: "issue bluenviron/5355 (Hanwha QNV-8080R missing end byte)",
		enc: []byte{
			0x81, 0xca, 0x00, 0x03,
			0x11, 0x22, 0x33, 0x44,
			0x01, 0x06, 'c', 'a', 'm', 'e', 'r', 'a',
		},
		dec: []rtcp.Packet{&rtcp.SourceDescription{
			Chunks: []rtcp.SourceDescriptionChunk{{
				Source: 0x11223344,
				Items: []rtcp.SourceDescriptionItem{{
					Type: rtcp.SDESCNAME,
					Text: "camera",
				}},
			}},
		}},
	},
	{
		name: "issue bluenviron/5355 (Hanwha QNV-8080R compound sender report and malformed sdes)",
		enc: []byte{
			0x80, 0xc8, 0x00, 0x06,
			0x00, 0x0b, 0x7f, 0xd5,
			0x12, 0x34, 0x56, 0x78,
			0x90, 0xab, 0xcd, 0xef,
			0x00, 0x00, 0xd4, 0x50,
			0x00, 0x00, 0x00, 0x01,
			0x00, 0x00, 0x00, 0x04,
			0x81, 0xca, 0x00, 0x03,
			0x11, 0x22, 0x33, 0x44,
			0x01, 0x06, 'c', 'a', 'm', 'e', 'r', 'a',
		},
		dec: []rtcp.Packet{
			&rtcp.SenderReport{
				SSRC:        753621,
				NTPTime:     0x1234567890abcdef,
				RTPTime:     54352,
				PacketCount: 1,
				OctetCount:  4,
			},
			&rtcp.SourceDescription{
				Chunks: []rtcp.SourceDescriptionChunk{{
					Source: 0x11223344,
					Items: []rtcp.SourceDescriptionItem{{
						Type: rtcp.SDESCNAME,
						Text: "camera",
					}},
				}},
			},
		},
	},
	{
		name: "truncated item payload",
		enc: []byte{
			0x81, 0xca, 0x00, 0x03,
			0x11, 0x22, 0x33, 0x44,
			0x01, 0x06, 'c', 'a', 'm',
		},
		error: "rtcp: packet too short",
	},
}

func TestUnmarshal(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			pkts, err := Unmarshal(ca.enc)

			if ca.error != "" {
				require.EqualError(t, err, ca.error)
				return
			}

			require.NoError(t, err)
			require.Equal(t, ca.dec, pkts)
		})
	}
}

func FuzzUnmarshal(f *testing.F) {
	for _, c := range cases {
		f.Add(string(c.enc))
	}

	f.Fuzz(func(t *testing.T, b string) {
		pkts, err := Unmarshal([]byte(b))
		if err != nil {
			return
		}

		for _, pkt := range pkts {
			_, err = pkt.Marshal()
			require.NoError(t, err)
		}
	})
}
