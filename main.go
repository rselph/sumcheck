package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
)

var verbose bool
var quiet bool
var tm bool
var chan_depth int
var buffSize int64
var ignoreMTime bool
var dbpath string
var dataRate float64

func main() {
	runtime.GOMAXPROCS(8)

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
	flag.StringVar(&dbpath, "db", "", "Path to database file")
	flag.Float64Var(&dataRate, "rate", 0.0, "Max data rate in bytes/sec")
	flag.Parse()

	// If we're rate limiting, and the buff flag was not explicitly set,
	// the cut down the buff size to 128k
	buffSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "buff" {
			buffSet = true
		}
	})

	if dataRate != 0.0 && !buffSet {
		buffSize /= 1024
	}

	switch {
	case flag.NArg() == 0:
		// set check_dir to cwd
		check_dir, err = os.Getwd()
		if err != nil {
			log.Println("Cannot get current working directory.  Please supply a directory as an argument.")
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
		flag.Usage()
		os.Exit(1)
	}

	check_dir = volPath(check_dir)

	if copy_dir == "" {
		if tm {
			// Set copy dir to latest tm backup
			copy_dir, err = getTMDir(check_dir)
			if err != nil {
				log.Println("Cannot get latest time machine directory.  Please supply a backup directory.")
				log.Fatal(err)
			}
		}
	} else {
		copy_dir = volPath(copy_dir)
	}

	if !quiet {
		fmt.Printf("Checking: %s\n", check_dir)
		if copy_dir != "" {
			fmt.Printf("Comparing with backup at: %s\n", copy_dir)
		}
	}

	if dbpath == "" {
		me, err := user.Current()
		if err != nil {
			log.Fatal(err)
		}

		dbpath = filepath.Join(me.HomeDir, ".sumcheck.sqlite3")
	}
	db, err := NewFileDB(dbpath)
	if err != nil {
		log.Println("Cannot connect to db at " + dbpath)
		log.Fatal(err)
	}
	defer db.Close()

	initWalkers()

	start := time.Now()

	final_out := make(chan *fileJob, chan_depth)
	defer close(final_out)

	if copy_dir == "" {
		in1_chan := make(chan *fileJob, chan_depth)
		defer close(in1_chan)
		out1_chan := make(chan *fileJob, chan_depth)
		defer close(out1_chan)

		go Walker(in1_chan, nil, check_dir, "")
		go Calculator(in1_chan, out1_chan, buffSize, dataRate)
		go dbChecker(out1_chan, final_out, db)
	} else {
		in1_chan := make(chan *fileJob, chan_depth)
		defer close(in1_chan)
		in2_chan := make(chan *fileJob, chan_depth)
		defer close(in2_chan)
		out1_chan := make(chan *fileJob, chan_depth)
		defer close(out1_chan)
		out2_chan := make(chan *fileJob, chan_depth)
		defer close(out2_chan)
		comp_out := make(chan *fileJob, chan_depth)
		defer close(comp_out)

		go Walker(in1_chan, in2_chan, check_dir, copy_dir)
		go Calculator(in1_chan, out1_chan, buffSize, dataRate/2.0)
		go Calculator(in2_chan, out2_chan, buffSize, dataRate/2.0)
		go Comparator(out1_chan, out2_chan, comp_out)
		go dbChecker(comp_out, final_out, db)
	}

	var totalIO int64
	var fileCount int64

	for f := range final_out {
		if f == nil {
			break
		}

		if f.IoLen >= 0 {
			totalIO += f.IoLen
			fileCount++
		}

		switch {
		case quiet:
			if f.Err != nil && f.Err.code == code_BAD_SUM {
				log.Println(f.Err.Error())
			}

		case verbose:
			if f.Err == nil {
				f.Err = NewError(code_OK, f, "")
			}
			log.Println(f.Err.Error())

		case f.Err != nil:
			if f.Err.code != code_NEW_SUM {
				log.Println(f.Err.Error())
			}
		}
	}

	stop := time.Now()

	if !quiet {
		elapsed := stop.Sub(start)
		fmt.Println()
		fmt.Printf("%v bytes in %v (%v bytes per sec.)\n",
			Eng(float64(totalIO)), prettyDuration(elapsed),
			Eng(float64(totalIO)/elapsed.Seconds()))
		fmt.Printf("%v files in %v (%v files per sec.)\n",
			Eng(float64(fileCount)), prettyDuration(elapsed),
			Eng(float64(fileCount)/elapsed.Seconds()))
	}
}

var re = regexp.MustCompile(`(.*)(\.[[:digit:]]+)(.*)`)

func prettyDuration(d time.Duration) string {
	raw := d.String()

	parts := re.FindStringSubmatch(raw)
	if parts == nil || parts[0] == "" {
		return raw
	}

	return parts[1] + parts[3]
}
