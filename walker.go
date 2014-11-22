// walker.go
package main

import (
	"os"
	"path/filepath"
	"regexp"
)

var excludeFiles = []string{
	`^.*-shm$`,
	`^\.DS_Store$`,
	`^.*.sqlite3$`,
}
var excludeFileRegexp []*regexp.Regexp

var excludeDirs = []string{
	`^.*/\.Trash$`,
	`^/Volumes/[^/]+/\.Spotlight-V100$`,
	`^/Volumes/[^/]+/\.DocumentRevisions-V100$`,
	`^/Volumes/[^/]+/\.MobileBackups$`,
	`^/Volumes/[^/]+/\.MobileBackups\.trash$`,
	`^/Volumes/[^/]+/\.Trashes$`,
	`^/Volumes/[^/]+/\.fseventsd$`,
	`^/Volumes/[^/]+/\.vol$`,
}
var excludeDirRegexp []*regexp.Regexp

var notEligible *myError = &myError{"File skipped"}

func initWalkers() {
	excludeFileRegexp = make([]*regexp.Regexp, len(excludeFiles))
	for i, v := range excludeFiles {
		excludeFileRegexp[i] = regexp.MustCompile(v)
	}

	excludeDirRegexp = make([]*regexp.Regexp, len(excludeDirs))
	for i, v := range excludeDirs {
		excludeDirRegexp[i] = regexp.MustCompile(v)
	}
}

func isEligible(path string, info os.FileInfo) (err error) {
	switch {
	case info == nil:
		err = notEligible

	case info.Mode().IsRegular():
		for _, re := range excludeFileRegexp {
			if re.MatchString(info.Name()) {
				//fmt.Println("Ingoring:", path)
				return notEligible
			}
		}

	case info.Mode().IsDir():
		err = notEligible
		for _, re := range excludeDirRegexp {
			if re.MatchString(path) {
				//fmt.Println("Ingoring Dir:", path)
				return filepath.SkipDir
			}
		}

	default:
		err = notEligible
	}

	return
}

type fileActor func(node string, info os.FileInfo)

func actionVisitor(node string, info os.FileInfo, err error, action fileActor) (returnError error) {
	if err != nil {
		return nil
	}

	returnError = isEligible(node, info)
	switch returnError {

	case nil:
		action(node, info)

	case filepath.SkipDir:
		break

	default:
		returnError = nil

	}

	return
}

func DualWalker(in1, in2 chan *fileJob, path1, path2 string) {
	defer func() { in1 <- nil }()
	defer func() { in2 <- nil }()

	prefix_len := len(path1)

	action := func(node string, info os.FileInfo) {
		in1 <- &fileJob{node, info, nil, 0}
		in2 <- &fileJob{filepath.Join(path2, node[prefix_len:]), nil, nil, 0}
	}

	fileVisitor := func(node string, info os.FileInfo, err error) error {
		return actionVisitor(node, info, err, action)
	}

	filepath.Walk(path1, fileVisitor)
}

func Walker(in chan *fileJob, path string) {
	defer func() { in <- nil }()

	action := func(node string, info os.FileInfo) {
		in <- &fileJob{node, info, nil, 0}
	}

	fileVisitor := func(node string, info os.FileInfo, err error) error {
		return actionVisitor(node, info, err, action)
	}

	filepath.Walk(path, fileVisitor)
}
