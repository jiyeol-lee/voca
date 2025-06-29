package vocabulary

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func (s *store) checkIsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = s.storePath
	err := cmd.Run()
	return err == nil
}

func (s *store) checkIsSynced() error {
	cmdRevParse := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmdRevParse.Dir = s.storePath
	revParseOutput, err := cmdRevParse.Output()
	if err != nil {
		return fmt.Errorf("error running git rev-parse: %w", err)
	}
	cmdRevList := exec.Command(
		"git",
		"rev-list",
		"--left-right",
		"--count",
		fmt.Sprintf("origin/%s...HEAD", strings.TrimSpace(string(revParseOutput))),
	)
	cmdRevList.Dir = s.storePath
	revListOutput, err := cmdRevList.Output()
	if err != nil {
		return fmt.Errorf("error running git rev-list: %w", err)
	}

	counts := strings.Fields(string(revListOutput))
	if len(counts) != 2 {
		return fmt.Errorf("unexpected output from git rev-list: %s", string(revListOutput))
	}
	if counts[0] != "0" || counts[1] != "0" {
		return fmt.Errorf("local repository is not synced with remote: %s", string(revListOutput))
	}
	return nil
}

func (s *store) commitChanges() error {
	formattedNow := time.Now().Format("2006-01-02 15:04:05 (-0700)")
	cmdAdd := exec.Command("git", "add", "-A")
	cmdAdd.Dir = s.storePath
	err := cmdAdd.Run()
	if err != nil {
		return fmt.Errorf("error running git add: %w", err)
	}

	cmdCommit := exec.Command(
		"git",
		"commit",
		"-m",
		fmt.Sprintf("chore: sync vocabulary at %s", formattedNow),
	)
	cmdCommit.Dir = s.storePath
	err = cmdCommit.Run()
	if err != nil {
		return fmt.Errorf("error running git commit: %w", err)
	}

	return nil
}

func (s *store) pushChanges() error {
	cmdPush := exec.Command("git", "push", "--force-with-lease")
	cmdPush.Dir = s.storePath
	err := cmdPush.Run()
	if err != nil {
		return fmt.Errorf("error running git push: %w", err)
	}

	return nil
}

func (s *store) syncStore() error {
	isGitRepo := s.checkIsGitRepo()
	if !isGitRepo {
		return fmt.Errorf("store is not a git repository")
	}
	err := s.checkIsSynced()
	if err != nil {
		return fmt.Errorf("store is not synced: %w", err)
	}

	err = s.commitChanges()
	if err != nil {
		return fmt.Errorf("error committing changes: %w", err)
	}

	err = s.pushChanges()
	if err != nil {
		return fmt.Errorf("error pushing changes: %w", err)
	}

	return nil
}
