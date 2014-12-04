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

type fileActor func(node string, info os.FileInfo)

func visitFileWithAction(node string, info os.FileInfo, err error, action fileActor) error {
	if err != nil || info == nil {
		return nil
	}

	switch {
	case info.Mode().IsRegular():
		for _, re := range excludeFileRegexp {
			if re.MatchString(info.Name()) {
				return nil
			}
		}
		action(node, info)

	case info.Mode().IsDir():
		for _, re := range excludeDirRegexp {
			if re.MatchString(node) {
				return filepath.SkipDir
			}
		}
	}

	return nil
}

func Walker(out1, out2 chan *fileJob, path1, path2 string) {
	defer func() { out1 <- nil }()

	path1, _ = filepath.EvalSymlinks(path1)

	var action fileActor
	if out2 == nil {
		action = func(node string, info os.FileInfo) {
			out1 <- &fileJob{node, info, nil, 0, 0}
		}
	} else {
		prefix_len := len(path1)
		action = func(node string, info os.FileInfo) {
			out1 <- &fileJob{node, info, nil, 0, 0}
			out2 <- &fileJob{filepath.Join(path2, node[prefix_len:]), nil, nil, 0, 0}
		}
		defer func() { out2 <- nil }()
	}

	filepath.Walk(path1, func(node string, info os.FileInfo, err error) error {
		return visitFileWithAction(node, info, err, action)
	})
}
