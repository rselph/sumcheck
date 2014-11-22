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
	Err    error
	Chksum uint64
}

func (f *fileJob) CalculateChecksum(h hash.Hash64, data []byte) {
	if f.Err != nil {
		return
	}

	if f.Info == nil {
		f.Info, f.Err = os.Stat(f.Fpath)
		if f.Err != nil {
			return
		}
	}

	var file *os.File
	file, f.Err = os.Open(f.Fpath)
	if f.Err != nil {
		return
	}
	defer file.Close()

	h.Reset()
	for {
		count, err := file.Read(data)
		if err != nil && err != io.EOF {
			f.Err = err
			return
		}
		if count == 0 {
			break
		}
		h.Write(data[:count])
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
