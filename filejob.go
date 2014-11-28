// filejob.go
package main

import (
	"hash"
	"hash/fnv"
	"io"
	"os"
)

type fileJob struct {
	Fpath  string
	Info   os.FileInfo
	Err    *myError
	Chksum uint64
	IoLen  int64 // Actual amount of bytes read for stats
}

func (f *fileJob) CalculateChecksum(h hash.Hash64, data []byte) {
	var err error

	if f.Err != nil {
		return
	}

	if f.Info == nil {
		f.Info, err = os.Stat(f.Fpath)
		f.Err = WrapError(err)
		if f.Err != nil {
			return
		}
	}

	var file *os.File
	file, err = os.Open(f.Fpath)
	f.Err = WrapError(err)
	if f.Err != nil {
		return
	}
	defer file.Close()

	h.Reset()
	for {
		count, err := file.Read(data)
		if err != nil && err != io.EOF {
			f.Err = WrapError(err)
			return
		}
		if count == 0 {
			break
		}
		h.Write(data[:count])
		f.IoLen += int64(count)
	}

	f.Chksum = h.Sum64()
}

func Calculator(in, out chan *fileJob, buffSize int64) {
	defer func() { out <- nil }()

	h := fnv.New64()
	data := make([]byte, buffSize)

	for f := range in {
		if f == nil {
			return
		}

		f.CalculateChecksum(h, data)
		out <- f
	}
}
