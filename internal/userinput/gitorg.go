package userinput

import (
	"bufio"
	"fmt"
	"io"
)

// GitOrg resolves GIT_CLONE_USER_NAME via the pre-resolved-value
// fast-path: when prearg is non-empty, return verbatim. The cmd layer
// sources it from --git-org / GIT_CLONE_USER_NAME. PERSONAL_COMPUTER
// fast-path: when personalComputer=true, return "Nivl" without
// prompting. Otherwise prompt with `What is the name of the git org? `
// and retry until non-empty.
func GitOrg(prearg string, in io.Reader, out io.Writer, personalComputer bool) (string, error) {
	if prearg != "" {
		return prearg, nil
	}
	if personalComputer {
		return "Nivl", nil
	}
	value, err := promptNonEmpty(bufio.NewReader(in), out, "What is the name of the git org?")
	if err != nil {
		return "", fmt.Errorf("non-empty prompt: %w", err)
	}
	return value, nil
}

// promptNonEmpty prints `<prompt> ` to out, reads one line from r, and
// retries on empty input. EOF returns errNoInput. Takes *bufio.Reader
// (not io.Reader) so callers that already own a bufio.Reader can pass
// it through without double-buffering — bufio.NewReader on an
// already-drained io.Reader would miss whatever the first bufio peeked
// ahead (see GitHost option-4 fall-through).
func promptNonEmpty(r *bufio.Reader, out io.Writer, prompt string) (string, error) {
	for {
		if _, err := fmt.Fprintf(out, "%s ", prompt); err != nil {
			return "", fmt.Errorf("write prompt: %w", err)
		}
		line, err := readLine(r)
		if err != nil {
			return "", fmt.Errorf("read line: %w", err)
		}
		if line != "" {
			return line, nil
		}
	}
}
