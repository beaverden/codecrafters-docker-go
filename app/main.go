package main

import (
	"fmt"
	"io"
	"path/filepath"

	// Uncomment this block to pass the first stage!
	"os"
	"os/exec"
)

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	// fmt.Println("Logs from your program will appear here!")

	// Uncomment this block to pass the first stage!
	//
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	dirpath, _ := os.MkdirTemp("", "test-run")
	original_path, err := os.Open(command)
	if err != nil {
		fmt.Printf("Failed to open original file: %v", err)
		os.Exit(1)
	}
	copied_path, err := os.OpenFile(filepath.Join(dirpath, "executable"), os.O_WRONLY|os.O_CREATE, 0777)
	if err != nil {
		fmt.Printf("Failed to open copy file location: %v", err)
		os.Exit(1)
	}
	io.Copy(copied_path, original_path)
	original_path.Close()
	copied_path.Close()

	all_args := []string{dirpath, "./executable"}
	all_args = append(all_args, args...)

	// fmt.Println("Args", all_args)
	cmd := exec.Command("chroot", all_args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}
	os.Exit(cmd.ProcessState.ExitCode())
}
