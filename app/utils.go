package main

import "os/exec"

func extractTarArchive(tarFile string, outputDir string) error {
	cmd := exec.Command("tar", "-xf", tarFile, "-C", outputDir)
	return cmd.Run()
}
