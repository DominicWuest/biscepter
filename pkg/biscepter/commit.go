package biscepter

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// getCommitsBetween returns the hashes of all commits between the passed good and bad commit.
// The hashes of the commits included in the avoidedCommits argument will not be included in the returned result.
// The passed commits are included in the result.
// The returned slice is ordered chronologically, starting from the good commit at index 0 and the bad commit at the last index
func getCommitsBetween(goodCommitHash, badCommitHash, repoPath string) ([]string, error) {
	cmd := exec.Command("git", "rev-list", "--reverse", "--first-parent", "^"+goodCommitHash, badCommitHash)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to get rev-list of bad commit %s to good commit %s", badCommitHash, goodCommitHash), err)
	}
	commits := strings.Split(string(out[:len(out)-1]), "\n")

	// Get excluded boundary commit
	cmd = exec.Command("git", "rev-parse", goodCommitHash)
	cmd.Dir = repoPath
	out, err = cmd.Output()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to get rev-list of bad commit %s to good commit %s", badCommitHash, goodCommitHash), err)
	}
	return append([]string{string(out)[:len(out)-1]}, commits...), nil
}

// getActualCommit returns the hash of the commit which the passed commit results in, given the passed replacements
func getActualCommit(commitHash string, commitReplacements *sync.Map) string {
	if val, ok := commitReplacements.Load(commitHash); ok {
		return getActualCommit(val.(string), commitReplacements)
	}
	return commitHash
}

// getMergedParent returns the commit hash of the current commit's parent which got merged, given the
// passed parent is on the branch the parent got merged on.
// If the current commit is not a merge commit or an octopus commit, getMergedParent returns an empty string
func getMergedParent(curCommitHash, parentCommitHash, repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", fmt.Sprintf("%s^@", curCommitHash))
	cmd.Dir = repoPath
	outBytes, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("couldn't check if commit is merge commit or not - %v", err)
	}
	out := string(outBytes)
	// Trim trailing newline
	parents := strings.Split(out[:len(out)-1], "\n")
	if len(parents) == 1 {
		return "", nil
	} else if len(parents) > 2 {
		// Octopus commit
		// TODO: Maybe implementable?
		return "", nil
	}
	// Merge commit!

	if parentCommitHash == parents[0] {
		return parents[1], nil
	} else if parentCommitHash == parents[1] {
		return parents[0], nil
	}

	return "", fmt.Errorf("passed parent commit %s is not actually a parent of %s (%s or %s)", parentCommitHash, curCommitHash, parents[0], parents[1])
}
