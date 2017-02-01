package utils

import (
	"errors"
	"io"
	"time"
)

const ChunkSize = 128 * 1024

// Copy copies from rd to wr until EOF or timeout is reached on rd or it was cancelled
func Copy(wr io.Writer, rd io.Reader, timeout time.Duration, cancel <-chan bool, chunkSize int, count int) (bool, error) {
	if chunkSize < 1 {
		return false, errors.New("Copy error: chunkSize can't be less than 1")
	}

	len := make(chan int)
	buf := make([]byte, chunkSize)
	readErrChan := make(chan error)

Loop:
	for i := 0; i != count; i++ {
		go func() {
			n, err := rd.Read(buf)
			if n == 0 && err != nil {
				if err != io.EOF {
					readErrChan <- err
				}
				close(len)
			} else {
				len <- n
			}
		}()

		select {
		case err, ok := <-readErrChan:
			if ok {
				close(readErrChan)
				return false, err
			}
		case _, ok := <-cancel:
			if ok {
				return true, nil
			}
		case <-time.After(timeout):
			return false, errors.New("timeout")
		case n, ok := <-len:
			if !ok {
				break Loop
			}

			_, err := wr.Write(buf[0:n])
			if err != nil {
				return false, err
			}
		}
	}

	return false, nil
}
