package main

import (
	"syscall"

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
	args := os.Args[3:len(os.Args)]

	jailPath, _ := os.MkdirTemp("", "test-run")

	// originalPath, err := os.Open(command)
	// if err != nil {
	// 	fmt.Printf("Failed to open original file: %v", err)
	// 	os.Exit(1)
	// }
	// copiedPath, err := os.OpenFile(filepath.Join(dirpath, "executable"), os.O_WRONLY|os.O_CREATE, 0777)
	// if err != nil {
	// 	fmt.Printf("Failed to open copy file location: %v", err)
	// 	os.Exit(1)
	// }
	// io.Copy(copiedPath, originalPath)
	// originalPath.Close()
	// copiedPath.Close()

	registry := NewRegistry(os.Args[2])
	if err := registry.Authenticate(); err != nil {
		panic(err)
	}
	if err := registry.Pull(jailPath); err != nil {
		panic(err)
	}

	all_args := []string{jailPath}
	all_args = append(all_args, args...)

	// fmt.Println("Args", all_args)
	cmd := exec.Command("chroot", all_args...)
	options := syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWPID}
	cmd.SysProcAttr = &options
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	_ = cmd.Run()
	os.Exit(cmd.ProcessState.ExitCode())
}
