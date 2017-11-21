package utils

import (
	"io"
	"reflect"
)

type LimitedReader struct {
	R io.ReadSeeker
	N int64
}

func (l *LimitedReader) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		return 0, io.EOF
	}

	if int64(len(p)) > l.N {
		p = p[0:l.N]
	}

	n, err = l.R.Read(p)
	l.N -= int64(n)

	return
}

func (l *LimitedReader) Seek(offset int64, whence int) (int64, error) {
	l.N = reflect.ValueOf(l.R).Elem().FieldByName("i").Int()
	return l.R.Seek(offset, whence)
}

func LimitReader(r io.ReadSeeker, n int64) io.ReadSeeker {
	return &LimitedReader{R: r, N: n}
}
