// comparejob.go
package main

type compareJob struct {
	f1, f2      *fileJob
	description string
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
			c.description = "ERROR: "

		case !ignoreMTime && c.f1.Info.ModTime().UnixNano() > c.f2.Info.ModTime().UnixNano():
			c.description = "ERROR: "
			c.f1.Err = &myError{"File has been changed since backup was made."}

		case c.f1.Info.Size() != c.f2.Info.Size() || c.f1.Chksum != c.f2.Chksum:
			c.description = "FAIL: "
			c.f1.Err = &myError{"file content mismatch"}
		}

		out <- c
	}
}
