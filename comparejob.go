// comparejob.go
package main

type compareJob struct {
	f1, f2 *fileJob
	err    *myError
}

func Comparator(in1, in2 chan *fileJob, out chan *compareJob) {
	var ok bool

	defer func() { out <- nil }()

	for {
		c := new(compareJob)

		c.f1, ok = <-in1
		if !ok || c.f1 == nil {
			return
		}
		c.f2, ok = <-in2
		if !ok || c.f2 == nil {
			return
		}

		switch {
		case c.f1.Err != nil || c.f2.Err != nil:
			break

		case !ignoreMTime && c.f1.Info.ModTime().After(c.f2.Info.ModTime()):
			c.err = NewError(code_NEWER, c.f1, "has changed since backup")

		case c.f1.Info.Size() != c.f2.Info.Size() || c.f1.Chksum != c.f2.Chksum:
			c.err = NewError(code_BAD_SUM, c.f1, "has different checksum in backup")
		}

		out <- c
	}
}
