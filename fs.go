package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func rootVolPath() string {
	volumes, err := filepath.Glob("/Volumes/*")
	if err != nil || volumes == nil {
		log.Println("Unable to list /Volumes/*")
		log.Fatal(err)
	}

	for _, volume := range volumes {
		link_path, err := os.Readlink(volume)
		if err == nil && link_path == "/" {
			return volume
		}
	}

	log.Fatal("Could not find root volume")
	return ""
}

func volPath(target string) string {
	var err error

	target, err = filepath.Abs(target)
	if err != nil {
		log.Fatal(err)
	}
	target = filepath.Clean(target)

	target_list := strings.Split(target, string(filepath.Separator))
	if len(target_list) >= 2 &&
		target_list[0] == "" &&
		target_list[1] == "Volumes" {
		return target
	}

	return filepath.Join(rootVolPath(), target)
}

func getTMDir() (dirname string, err error) {
	dirname_bytes, err := exec.Command("tmutil", "machinedirectory").CombinedOutput()
	if err == nil {
		dirname = filepath.Join(strings.TrimSpace(string(dirname_bytes)), "Latest")
	}
	return
}
