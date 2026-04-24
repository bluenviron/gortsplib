package base

import (
	"bufio"
	"fmt"
)

func readByteEqual(rb *bufio.Reader, cmp byte) error {
	byt, err := rb.ReadByte()
	if err != nil {
		return err
	}

	if byt != cmp {
		return fmt.Errorf("expected '%c', got '%c'", cmp, byt)
	}

	return nil
}

func readBytesLimited(rb *bufio.Reader, delim byte, n int) ([]byte, error) {
	for i := 1; i <= n; i++ {
		byts, err := rb.Peek(i)
		if err != nil {
			return nil, err
		}

		if byts[len(byts)-1] == delim {
			rb.Discard(len(byts)) //nolint:errcheck
			return byts, nil
		}
	}
	return nil, fmt.Errorf("buffer length exceeds %d", n)
}

func readBytesLimitedUntilSpaceOrCarriage(rb *bufio.Reader, n int) ([]byte, error) {
	for i := 1; i <= n; i++ {
		byts, err := rb.Peek(i)
		if err != nil {
			return nil, err
		}

		last := byts[len(byts)-1]
		if last == ' ' || last == '\r' {
			rb.Discard(len(byts)) //nolint:errcheck
			return byts, nil
		}
	}
	return nil, fmt.Errorf("buffer length exceeds %d", n)
}
