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

func (f *fileJob) Stat() {
	var err error

	if f.Info == nil {
		f.Info, err = os.Stat(f.Fpath)
		f.Err = WrapError(err)
	}
}

func (f *fileJob) CalculateChecksum(
	h hash.Hash64,
	data []byte,
	tr *ReadThrottler) {

	var err error

	if f.Err != nil {
		return
	}

	f.Stat()
	if f.Err != nil {
		return
	}

	var file *os.File
	file, err = os.Open(f.Fpath)
	f.Err = WrapError(err)
	if f.Err != nil {
		return
	}
	defer file.Close()

	h.Reset()
	tr.SetReader(file)
	for {
		count, err := tr.Read(data)
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

func Calculator(in, out chan *fileJob, buffSize int64, rate float64) {
	defer func() { out <- nil }()

	h := fnv.New64()
	data := make([]byte, buffSize)

	var reader *ReadThrottler
	if rate == 0.0 {
		reader = NewReadThrottler(nil)
	} else {
		t := new(Throttler)
		t.Start(rate)
		reader = NewReadThrottler(t)
	}

	for f := range in {
		if f == nil {
			return
		}

		f.CalculateChecksum(h, data, reader)
		out <- f
	}
}
