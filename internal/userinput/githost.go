package userinput

import (
	"bufio"
	"fmt"
	"io"
)

// GitHost resolves GIT_HOST via the pre-resolved-value fast-path: when
// prearg is non-empty, return verbatim. The cmd layer sources it from
// --git-host / GIT_HOST. Otherwise prompt with a 4-option menu;
// dispatch on the first byte (1/2/3/4); invalid input loops with
// `Invalid value` to out. Option 4 (`Custom`) falls through to a
// non-empty free-form prompt.
func GitHost(prearg string, in io.Reader, out io.Writer) (string, error) {
	if prearg != "" {
		return prearg, nil
	}

	r := bufio.NewReader(in)
	for {
		if _, err := fmt.Fprint(out, "Pick the git server\n\t1) GitHub\n\t2) Bitbucket\n\t3) Gitlab\n\t4) Custom\n"); err != nil {
			return "", fmt.Errorf("write menu: %w", err)
		}
		line, err := readLine(r)
		if err != nil {
			return "", fmt.Errorf("read line: %w", err)
		}
		switch firstByte(line) {
		case '1':
			return "git@github.com", nil
		case '2':
			return "git@bitbucket.org", nil
		case '3':
			return "git@gitlab.com", nil
		case '4':
			value, err := promptNonEmpty(r, out, "Type the URL of the git server (usually in the form of git@github.com):")
			if err != nil {
				return "", fmt.Errorf("non-empty prompt: %w", err)
			}
			return value, nil
		}
		if _, err := fmt.Fprintln(out, "Invalid value"); err != nil {
			return "", fmt.Errorf("write retry hint: %w", err)
		}
	}
}
