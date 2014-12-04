package main

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
)

func volPath(target string) string {
	var err error

	target, err = filepath.Abs(target)
	if err != nil {
		log.Fatal(err)
	}
	target = filepath.Clean(target)

	target_list := filepath.SplitList(target)
	if target_list[0] == "Volumes" {
		return target
	}

	volName := ""

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
		fmt.Println("reading output of diskutil")
		log.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
	if volName == "" {
		log.Fatal("Unable to determine root volume.")
	}

	return filepath.Join("/Volumes", volName, target)
}

func getTMDir() (dirname string, err error) {
	dirname_bytes, err := exec.Command("tmutil", "machinedirectory").CombinedOutput()
	if err != nil {
		dirname = filepath.Join(strings.TrimSpace(string(dirname_bytes)), "Latest")
	}
	return
}
