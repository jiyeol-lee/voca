package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func pagerView(content string) error {
	tmpDir := os.TempDir()
	// write the content to a temporary file
	// file name is generated using a random string to avoid conflicts
	tmpFile, err := os.CreateTemp(tmpDir, "pager-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	// write the content to the temporary file
	_, err = tmpFile.WriteString(content)
	if err != nil {
		return err
	}
	tmpFile.Close()

	awkScript := `
{
	if ($0 ~ /^###### /) {
		print "\033[1;35m" $0 "\033[0m"
	} else if ($0 ~ /^##### /) {
		print "\033[1;36m" $0 "\033[0m"
	} else if ($0 ~ /^#### /) {
		print "\033[1;34m" $0 "\033[0m"
	} else if ($0 ~ /^### /) {
		print "\033[1;32m" $0 "\033[0m"
	} else if ($0 ~ /^## /) {
		print "\033[1;33m" $0 "\033[0m"
	} else if ($0 ~ /^# /) {
		print "\033[1;35m" $0 "\033[0m"
	} else {
		print $0
	}
}
`

	cmd := exec.Command(
		"sh",
		"-c",
		fmt.Sprintf(`awk '%s' %s | less -Rc`, awkScript, tmpFile.Name()),
	)
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
