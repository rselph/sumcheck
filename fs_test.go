package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func getRootVolForTest(t *testing.T) string {
	volName := ""

	cmd := exec.Command("diskutil", "info", "/")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Log(err)
		return ""
	}
	defer stdout.Close()

	if err := cmd.Start(); err != nil {
		t.Log(err)
		return ""
	}
	stdlines := bufio.NewScanner(stdout)
	for stdlines.Scan() {
		line := stdlines.Text()
		if strings.HasPrefix(line, "   Volume Name:") {
			volName = strings.Fields(line)[2]
		}
	}
	if err := stdlines.Err(); err != nil {
		t.Log(err)
		return ""
	}
	if err := cmd.Wait(); err != nil {
		t.Log(err)
		return ""
	}

	return volName
}

func checkPath(t *testing.T, root, testPath, expected string) {
	result := volPath(testPath)
	expected = fmt.Sprintf(expected, root)
	if result != expected {
		t.Errorf("Expected %s got %s", expected, result)
	}
	if !t.Failed() {
		t.Log("Tested", result)
	}
}

func TestFS(t *testing.T) {
	root := getRootVolForTest(t)
	if root == "" {
		t.Log("Skipping fs.go test: root volume not found.")
		t.Fail()
		return
	}
	t.Log("Using root:", root)

	cwd, err := os.Getwd()
	if err != nil {
		t.Log("Skipping fs.go test: current working directory not found.")
		t.Fail()
		return
	}
	t.Log("Using working directory:", cwd)

	me, err := user.Current()
	if err != nil {
		t.Log("Skipping fs.go test: home directory not found.")
		t.Fail()
		return
	}
	home := me.HomeDir
	t.Log("Using home directory:", home)

	checkPath(t, root, "/usr/bin", "/Volumes/%s/usr/bin")
	checkPath(t, root, cwd, "/Volumes/%s"+cwd)
	checkPath(t, root, ".", "/Volumes/%s"+cwd)
	checkPath(t, root, "../"+filepath.Base(cwd), "/Volumes/%s"+cwd)
	checkPath(t, root, home, "/Volumes/%s"+home)
	checkPath(t, root, "/", "/Volumes/%s")

	checkPath(t, "", "/Volumes/foo/bar", "%s/Volumes/foo/bar")
	checkPath(t, "", "/Volumes/foo", "%s/Volumes/foo")
}
