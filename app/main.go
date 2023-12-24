package main

import (
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

func setupLogging() {
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		lvl = "error"
	}
	ll, err := logrus.ParseLevel(lvl)
	if err != nil {
		ll = logrus.ErrorLevel
	}
	logrus.SetLevel(ll)
}

func main() {
	setupLogging()

	imageReference := os.Args[2]
	runArgs := os.Args[3:len(os.Args)]
	log.Infof("Running image %s with args [%v]", imageReference, strings.Join(runArgs, ", "))

	jailPath, _ := os.MkdirTemp("", "run-jail")
	log.Infof("Created jail path at %s", jailPath)

	registry := NewRegistry(imageReference)
	if err := registry.Authenticate(); err != nil {
		panic(err)
	}
	if err := registry.Pull(jailPath); err != nil {
		panic(err)
	}

	chrootArgs := []string{jailPath}
	chrootArgs = append(chrootArgs, runArgs...)
	log.Infof("Running chroot with args [%v]", strings.Join(chrootArgs, ", "))
	cmd := exec.Command("chroot", chrootArgs...)
	options := syscall.SysProcAttr{Cloneflags: syscall.CLONE_NEWPID}
	cmd.SysProcAttr = &options
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		panic(err)
	}
	log.Infof("Process finished with code: %d", cmd.ProcessState.ExitCode())
	os.Exit(cmd.ProcessState.ExitCode())
}
