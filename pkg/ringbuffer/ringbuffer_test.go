package ringbuffer

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateError(t *testing.T) {
	_, err := New(1000)
	require.EqualError(t, err, "size must be a power of two")
}

func TestPushBeforePull(t *testing.T) {
	r, err := New(1024)
	require.NoError(t, err)
	defer r.Close()

	ok := r.Push(bytes.Repeat([]byte{1, 2, 3, 4}, 1024/4))
	require.Equal(t, true, ok)

	ret, ok := r.Pull()
	require.Equal(t, true, ok)
	require.Equal(t, bytes.Repeat([]byte{1, 2, 3, 4}, 1024/4), ret)
}

func TestPullBeforePush(t *testing.T) {
	r, err := New(1024)
	require.NoError(t, err)
	defer r.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		ret, ok := r.Pull()
		require.Equal(t, true, ok)
		require.Equal(t, bytes.Repeat([]byte{1, 2, 3, 4}, 1024/4), ret)
	}()

	time.Sleep(100 * time.Millisecond)

	ok := r.Push(bytes.Repeat([]byte{1, 2, 3, 4}, 1024/4))
	require.Equal(t, true, ok)

	<-done
}

func TestClose(t *testing.T) {
	r, err := New(1024)
	require.NoError(t, err)

	ok := r.Push([]byte{1, 2, 3, 4})
	require.Equal(t, true, ok)

	_, ok = r.Pull()
	require.Equal(t, true, ok)

	ok = r.Push([]byte{5, 6, 7, 8})
	require.Equal(t, true, ok)

	r.Close()

	_, ok = r.Pull()
	require.Equal(t, false, ok)

	r.Reset()

	ok = r.Push([]byte{9, 10, 11, 12})
	require.Equal(t, true, ok)

	data, ok := r.Pull()
	require.Equal(t, true, ok)
	require.Equal(t, []byte{9, 10, 11, 12}, data)
}

func TestOverflow(t *testing.T) {
	r, err := New(32)
	require.NoError(t, err)

	for i := 0; i < 32; i++ {
		r.Push([]byte{1, 2, 3, 4})
	}

	ok := r.Push([]byte{5, 6, 7, 8})
	require.Equal(t, false, ok)

	for i := 0; i < 32; i++ {
		data, ok := r.Pull()
		require.Equal(t, true, ok)
		require.Equal(t, []byte{1, 2, 3, 4}, data)
	}
}

func BenchmarkPushPullContinuous(b *testing.B) {
	r, _ := New(1024 * 8)
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
	r, _ := New(128)
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
	r, _ := New(128)
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
