package gortsplib

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAsyncProcessorCloseAfterError(t *testing.T) {
	p := &asyncProcessor{bufferSize: 8}
	p.initialize()

	p.push(func() error {
		return fmt.Errorf("ok")
	})

	p.start()

	<-p.chStopped
	require.EqualError(t, p.stopError, "ok")

	p.close()
}
