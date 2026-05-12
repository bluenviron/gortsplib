package gortsplib

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// a reader can be in st.readers before SETUP has assigned setuppedTransport.
func TestServerStreamCloseWithUnsetupReader(t *testing.T) {
	stream := &ServerStream{
		readers:              make(map[*ServerSession]struct{}),
		activeUnicastReaders: make(map[*ServerSession]struct{}),
	}

	ss := &ServerSession{}
	ss.ctx, ss.ctxCancel = context.WithCancel(context.Background())
	stream.readers[ss] = struct{}{}

	require.NotPanics(t, func() {
		stream.Close()
	})
}
