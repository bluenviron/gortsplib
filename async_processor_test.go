package gortsplib

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAsyncProcessorStopAfterError(t *testing.T) {
	p := &asyncProcessor{bufferSize: 8}
	p.initialize()

	p.push(func() error {
		return fmt.Errorf("ok")
	})

	p.start()

	<-p.stopped
	require.EqualError(t, p.stopError, "ok")

	p.stop()
}
