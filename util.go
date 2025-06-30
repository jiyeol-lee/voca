package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
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

	var termWidth int
	if widthCmd := exec.Command("tput", "cols"); widthCmd != nil {
		if output, err := widthCmd.Output(); err == nil {
			if width := strings.TrimSpace(string(output)); width != "" {
				if parsedWidth, err := strconv.Atoi(width); err == nil && parsedWidth > 0 {
					termWidth = parsedWidth
				}
			}
		}
	}

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
	var cmd *exec.Cmd
	if termWidth > 0 {
		cmd = exec.Command(
			"sh",
			"-c",
			fmt.Sprintf(
				`fold -s -w %d %s | awk '%s' | less -Rc`,
				200,
				tmpFile.Name(),
				awkScript,
			),
		)
	} else {
		cmd = exec.Command(
			"sh",
			"-c",
			fmt.Sprintf(`awk '%s' %s | less -Rc`, awkScript, tmpFile.Name()),
		)
	}
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
