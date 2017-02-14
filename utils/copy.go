package utils

import (
	"errors"
	"io"
	"time"

	"bitbucket.org/ossystems/agent/libarchive"
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

// FIXME: is this the same algorithm as above? if yes, transform
// libarchive.Archive to implement the io.Reader api so we can merge
// the algorithms
func LACopy(la libarchive.Api, target io.Writer, sourcePath string, chunkSize int, skip int, seek int, count int, truncate bool) error {
	a := la.NewRead()
	defer la.ReadFree(a)

	la.ReadSupportFilterAll(a)
	la.ReadSupportFormatRaw(a)
	la.ReadSupportFormatEmpty(a)

	err := la.ReadOpenFileName(a, sourcePath, chunkSize)
	if err != nil {
		return err
	}

	e := libarchive.ArchiveEntry{}
	err = la.ReadNextHeader(a, e)

	// empty file special case
	if err == io.EOF {
		_, err := target.Write([]byte(""))
		if err != nil {
			return err
		}

		return nil
	}

	if err != nil {
		return err
	}

	toSkip := skip
	looped := 0
	for looped != count {
		data := make([]byte, chunkSize)
		bytesRead, err := la.ReadData(a, data, chunkSize)

		if err != nil {
			return err
		}

		if bytesRead == 0 {
			break
		}

		if toSkip > 0 {
			toSkip--
		} else {
			dataToBeWritten := make([]byte, bytesRead)
			copy(dataToBeWritten, data)
			_, err := target.Write(dataToBeWritten)
			if err != nil {
				return err
			}

			looped++
		}
	}

	return nil
}
