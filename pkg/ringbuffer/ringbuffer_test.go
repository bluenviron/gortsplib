package ringbuffer

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPushBeforePull(t *testing.T) {
	r := New(1024)
	defer r.Close()

	data := make([]byte, 1024)
	rand.Read(data)

	r.Push(data)
	ret, ok := r.Pull()
	require.Equal(t, true, ok)
	require.Equal(t, data, ret)
}

func TestPullBeforePush(t *testing.T) {
	r := New(1024)
	defer r.Close()

	data := make([]byte, 1024)
	rand.Read(data)

	done := make(chan struct{})
	go func() {
		defer close(done)
		ret, ok := r.Pull()
		require.Equal(t, true, ok)
		require.Equal(t, data, ret)
	}()

	time.Sleep(100 * time.Millisecond)

	r.Push(data)
	<-done
}

func BenchmarkPushPullContinuous(b *testing.B) {
	r := New(1024 * 8)
	defer r.Close()

	data := make([]byte, 1024)

	for n := 0; n < b.N; n++ {
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 0; i < 1024*8; i++ {
				r.Push(data)
			}
		}()

		for i := 0; i < 1024*8; i++ {
			r.Pull()
		}

		<-done
	}
}

func BenchmarkPushPullPaused5(b *testing.B) {
	r := New(128)
	defer r.Close()

	data := make([]byte, 1024)

	for n := 0; n < b.N; n++ {
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 0; i < 128; i++ {
				r.Push(data)
				time.Sleep(5 * time.Millisecond)
			}
		}()

		for i := 0; i < 128; i++ {
			r.Pull()
		}

		<-done
	}
}

func BenchmarkPushPullPaused10(b *testing.B) {
	r := New(1024 * 8)
	defer r.Close()

	data := make([]byte, 1024)

	for n := 0; n < b.N; n++ {
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 0; i < 128; i++ {
				r.Push(data)
				time.Sleep(10 * time.Millisecond)
			}
		}()

		for i := 0; i < 128; i++ {
			r.Pull()
		}

		<-done
	}
}
