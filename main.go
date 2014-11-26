// tmverify project main.go
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var verbose bool
var quiet bool
var tm bool
var chan_depth int
var buffSize int64
var ignoreMTime bool

func main() {
	runtime.GOMAXPROCS(4)

	var copy_dir string
	var check_dir string
	var err error

	flag.Usage = func() {
		fmt.Printf("Usage of %s:\n", os.Args[0])
		fmt.Printf("%s [-q] [-v] [-tm] [-m] [-depth n] [-buff n] [source [dest]]\n", os.Args[0])
		fmt.Println()
		fmt.Println("This utility checks the contents of files against themselves")
		fmt.Println("and backup copies to try to detect bit failures on disk.  In its")
		fmt.Println("most basic form (with no arguments), it will read all of the files")
		fmt.Println("in the current directory recursively.  As it reads the files, it")
		fmt.Println("creates a database of the files' checksums and modification times.")
		fmt.Println("The next time it is run, any file with the same modification time,")
		fmt.Println("but a different checksum will be flagged.")
		fmt.Println()
		fmt.Println("Given a source parameter, it will check that directory instead.")
		fmt.Println("Given a source and dest, it will do the same database checks,")
		fmt.Println("but also compare the contents of source recursively with dest.")
		fmt.Println()
		fmt.Println("The -tm flag causes it to use the TimeMachine directory that")
		fmt.Println("corresponds to source as the dest.")
		fmt.Println()
		fmt.Println("The -q flag will suppress all output except possible bit failures.")
		fmt.Println()
		fmt.Println("The -v flag will print information about all files checked, even")
		fmt.Println("the ones that are OK.")
		fmt.Println()
		fmt.Println("Normally, if the modification time of a file has changed, it")
		fmt.Println("is not considered a problem if the contents are different.")
		fmt.Println("If the -m flag is given, then files are compared regardless.")
		fmt.Println()
		fmt.Println("The --depth and --buff flags are for tuning.  The default values")
		fmt.Println("should work well in most situations.")
		fmt.Println()

		flag.PrintDefaults()
	}

	flag.CommandLine.SetOutput(os.Stdout)
	flag.BoolVar(&verbose, "v", false, "Print info about files that are OK")
	flag.BoolVar(&quiet, "q", false, "Only print info about checksum errors.  Other file access problems are ignored.")
	flag.BoolVar(&tm, "tm", false, "Compare against TimeMachine")
	flag.BoolVar(&ignoreMTime, "m", false, "Compare files, even if mtime is different.")
	flag.IntVar(&chan_depth, "depth", 10, "Work queue depth")
	flag.Int64Var(&buffSize, "buff", 128*1024*1024, "Size of IO buffer")
	flag.Parse()

	switch {
	case flag.NArg() == 0:
		// set check_dir to cwd
		check_dir, err = os.Getwd()
		if err != nil {
			fmt.Println("Cannot get current working directory.  Please supply a directory as an argument.")
			log.Fatal(err)
		}

	case flag.NArg() == 1:
		// use arg as check dir
		check_dir = flag.Arg(0)

	case flag.NArg() == 2:
		// use arg as check dir
		check_dir = flag.Arg(0)
		copy_dir = flag.Arg(1)

	default:
		// print usage, exit
		fmt.Println("Supply one or two directories")
		os.Exit(1)
	}

	check_dir, err = filepath.Abs(check_dir)
	if err != nil {
		fmt.Println("Cannot get absolute path of directory to check.")
		log.Fatal(err)
	}
	check_vol, check_path := getVolNameAndPath(check_dir)
	check_dir = filepath.Join("/Volumes", check_vol, check_path)

	if copy_dir == "" {
		if tm {
			// Set copy dir to latest tm backup
			copy_dir_bytes, err := exec.Command("tmutil", "machinedirectory").CombinedOutput()
			if err != nil {
				fmt.Println("Cannot get latest time machine directory.  Please supply a backup directory.")
				fmt.Println(copy_dir)
				log.Fatal(err)
			}
			copy_dir = strings.TrimSpace(string(copy_dir_bytes)) + "/Latest"

			copy_dir = filepath.Join(copy_dir, check_vol, check_path)
		}
	} else {
		copy_dir, err = filepath.Abs(copy_dir)
		if err != nil {
			fmt.Println("Cannot get absolute path of copy directory.")
			log.Fatal(err)
		}
	}

	if !quiet {
		fmt.Printf("Checking: %s\n", check_dir)
		if copy_dir != "" {
			fmt.Printf("Comparing with backup at: %s\n", copy_dir)
		}
	}

	db, err := newDBConnection()
	if err != nil {
		fmt.Println("Cannot connect to db at ~/.tmverify.sqlite3")
		log.Fatal(err)
	}
	defer closeDBConnection(db)

	initWalkers()

	db_out := make(chan *compareJob, chan_depth)
	defer close(db_out)

	if copy_dir == "" {
		in1_chan := make(chan *fileJob, chan_depth)
		defer close(in1_chan)
		out1_chan := make(chan *fileJob, chan_depth)
		defer close(out1_chan)

		go Walker(in1_chan, nil, check_dir, "")
		go Calculator(in1_chan, out1_chan, buffSize)
		go dbFileChecker(out1_chan, db_out, db)
	} else {
		in1_chan := make(chan *fileJob, chan_depth)
		defer close(in1_chan)
		in2_chan := make(chan *fileJob, chan_depth)
		defer close(in2_chan)
		out1_chan := make(chan *fileJob, chan_depth)
		defer close(out1_chan)
		out2_chan := make(chan *fileJob, chan_depth)
		defer close(out2_chan)
		comp_out := make(chan *compareJob, chan_depth)
		defer close(comp_out)

		go Walker(in1_chan, in2_chan, check_dir, copy_dir)
		go Calculator(in1_chan, out1_chan, buffSize)
		go Calculator(in2_chan, out2_chan, buffSize)
		go Comparator(out1_chan, out2_chan, comp_out)
		go dbCompareChecker(comp_out, db_out, db)
	}

	for c := range db_out {
		if c == nil {
			break
		}

		if c.err != nil {
			fmt.Println(c.err.Error())
		}

		if c.f1.Err != nil {
			if !quiet || c.f1.Err.code == code_BAD_SUM {
				fmt.Println(c.f1.Err.Error())
			}
		} else if verbose && c.err == nil {
			fmt.Println(NewError(code_OK, c.f1, "").Error())
		}

		if c.f2 != nil && c.f2.Err != nil {
			if !quiet || c.f2.Err.code == code_BAD_SUM {
				fmt.Println(c.f2.Err.Error())
			}
		}
	}
}

func getVolNameAndPath(target string) (volName, path string) {
	target = filepath.ToSlash(target)
	if strings.HasPrefix(target, "/Volumes/") {
		targ_dirs := strings.Split(target, "/")
		volName = targ_dirs[2]
		path = strings.Join(targ_dirs[3:], "/")
	} else {
		volName = "*"

		cmd := exec.Command("diskutil", "info", "/")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatal(err)
		}
		defer stdout.Close()

		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}
		stdlines := bufio.NewScanner(stdout)
		for stdlines.Scan() {
			line := stdlines.Text()
			if strings.HasPrefix(line, "   Volume Name:") {
				volName = strings.Fields(line)[2]
			}
		}
		if err := stdlines.Err(); err != nil {
			fmt.Println("reading output of diskutil:", err)
			log.Fatal(err)
		}
		if err := cmd.Wait(); err != nil {
			log.Fatal(err)
		}

		path = target[1:]
	}

	path = filepath.FromSlash(path)
	return
}

type myError struct {
	code myErrorCode
	info string
}

func (e *myError) Error() (output string) {
	switch e.code {
	case code_OK:
		output = "ok"

	case code_SKIPPED:
		output = "skipped"

	case code_NEW_SUM:
		output = "new checksum"

	case code_BAD_SUM:
		output = "BAD CHECKSUM"

	case code_NOT_FOUND:
		output = "file not found"

	case code_NEWER:
		output = "file newer"

	case code_OTHER:
		output = "error"
	}

	if e.info != "" {
		output += ": "
		output += e.info
	}
	return
}

func NewError(code myErrorCode, f *fileJob, info string) (err *myError) {
	if f != nil {
		err = &myError{code, f.Fpath + " " + info}
	} else {
		err = &myError{code, info}
	}
	return err
}

func WrapError(err error) (myerr *myError) {
	if err == nil {
		return nil
	}
	return &myError{code_OTHER, err.Error()}
}

type myErrorCode int

const (
	code_OK myErrorCode = iota
	code_SKIPPED
	code_NEW_SUM
	code_BAD_SUM
	code_NOT_FOUND
	code_NEWER
	code_OTHER
)
