package asyncprocessor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloseBeforeStart(_ *testing.T) {
	p := &Processor{
		BufferSize: 8,
	}
	p.Initialize()
	defer p.Close()
}

func TestCloseAfterError(t *testing.T) {
	done := make(chan struct{})

	p := &Processor{
		BufferSize: 8,
		OnError: func(_ context.Context, err error) {
			require.EqualError(t, err, "ok")
			close(done)
		},
	}
	p.Initialize()
	defer p.Close()

	p.Push(func() error {
		return fmt.Errorf("ok")
	})

	p.Start()

	<-done
}

func TestCloseBeforeError(_ *testing.T) {
	p := &Processor{
		BufferSize: 8,
		OnError:    func(_ context.Context, _ error) {},
	}
	p.Initialize()
	defer p.Close()

	p.Push(func() error {
		return nil
	})

	p.Start()
}

func TestCloseDuringError(_ *testing.T) {
	p := &Processor{
		BufferSize: 8,
		OnError: func(ctx context.Context, _ error) {
			<-ctx.Done()
		},
	}
	p.Initialize()
	defer p.Close()

	p.Push(func() error {
		return fmt.Errorf("ok")
	})

	p.Start()
}
