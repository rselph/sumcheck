// comparejob.go
package main

func Comparator(in1, in2 chan *fileJob, out chan *fileJob) {
	defer func() { out <- nil }()

	for {
		f1, ok := <-in1
		if !ok || f1 == nil {
			return
		}
		f2, ok := <-in2
		if !ok || f2 == nil {
			return
		}

		switch {
		case f1.Err != nil || f2.Err != nil:
			break

		case !ignoreMTime && f1.Info.ModTime().After(f2.Info.ModTime()):
			outErr := NewError(code_NEWER, f1, "has changed since backup")
			out <- &fileJob{f1.Fpath, f1.Info, outErr, f1.Chksum}

		case f1.Info.Size() != f2.Info.Size() || f1.Chksum != f2.Chksum:
			outErr := NewError(code_BAD_SUM, f1, "has different checksum in backup")
			out <- &fileJob{f1.Fpath, f1.Info, outErr, f1.Chksum}
		}

		out <- f1
		out <- f2
	}
}
