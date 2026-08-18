package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
)

func TestServerStreamH264PacketizationMode0TCPOK(t *testing.T) {
	s := &Server{
		RTSPAddress: "localhost:8554",
	}
	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	forma := &format.H264{
		PayloadTyp:        96,
		PacketizationMode: 0,
	}

	stream := &ServerStream{
		Server: s,
		Desc: &description.Session{Medias: []*description.Media{{
			Type:    description.MediaTypeVideo,
			Formats: []format.Format{forma},
		}}},
	}
	err = stream.Initialize()
	require.NoError(t, err)
	defer stream.Close()
}

func TestServerStreamH264PacketizationMode0UDPReject(t *testing.T) {
	s := &Server{
		RTSPAddress:    "localhost:8555",
		UDPRTPAddress:  "127.0.0.1:8002",
		UDPRTCPAddress: "127.0.0.1:8003",
	}
	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	forma := &format.H264{
		PayloadTyp:        96,
		PacketizationMode: 0,
	}

	stream := &ServerStream{
		Server: s,
		Desc: &description.Session{Medias: []*description.Media{{
			Type:    description.MediaTypeVideo,
			Formats: []format.Format{forma},
		}}},
	}
	err = stream.Initialize()
	require.Error(t, err)
	require.EqualError(t, err, "H264 packetization-mode=0 is not supported by the server")
}
