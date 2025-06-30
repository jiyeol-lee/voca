package main

import (
	"io"
	"os"
	"os/exec"
	"strings"
)

func pagerView(content string) error {
	cmd := exec.Command("less", "-R")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	_, err = io.Copy(stdin, strings.NewReader(content))
	if err != nil {
		return err
	}
	err = stdin.Close()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}
