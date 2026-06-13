package userinput

import (
	"bufio"
	"fmt"
	"io"
)

// InstallRetryDecision enumerates the answers to the failed-packages
// prompt shown when `install packages` finishes with failures.
type InstallRetryDecision int

const (
	// InstallRetryAbort stops the whole run with an error.
	InstallRetryAbort InstallRetryDecision = iota + 1
	// InstallRetryAgain re-attempts only the failed packages.
	InstallRetryAgain
	// InstallRetryIgnore accepts the failures and continues.
	InstallRetryIgnore
)

// InstallRetryChoice resolves what to do about packages that failed to
// install or upgrade. Pre-resolved-value fast-path: when prearg is
// non-empty it is parsed as abort|retry|ignore and the prompt is
// skipped — the cmd layer resolves --install-failure-resolution /
// INSTALL_FAILURE_RESOLUTION and validates the value before calling.
// Otherwise prompts on out and reads a 1/2/3 answer from in. Anything
// else reprompts; EOF returns errNoInput so unattended runs fail
// loudly instead of looping forever. Callers that prompt repeatedly
// should pass the same *bufio.Reader each time (bufio.NewReader
// returns it unchanged) so buffered input survives across asks.
func InstallRetryChoice(prearg string, in io.Reader, out io.Writer) (InstallRetryDecision, error) {
	if prearg != "" {
		return parseInstallRetry(prearg)
	}
	r := bufio.NewReader(in)
	for {
		_, err := fmt.Fprint(out, "\nSome packages failed to install or upgrade. What do you want to do?\n"+
			"  1) Abort\n"+
			"  2) Try again (retries only the failed packages)\n"+
			"  3) Ignore the failures and continue\n"+
			"Choice (1/2/3)? ")
		if err != nil {
			return 0, fmt.Errorf("write prompt: %w", err)
		}
		line, err := readLine(r)
		if err != nil {
			return 0, fmt.Errorf("read line: %w", err)
		}
		switch firstByte(line) {
		case '1':
			return InstallRetryAbort, nil
		case '2':
			return InstallRetryAgain, nil
		case '3':
			return InstallRetryIgnore, nil
		}
		if _, err := fmt.Fprintln(out, "Invalid value"); err != nil {
			return 0, fmt.Errorf("write retry hint: %w", err)
		}
	}
}

// parseInstallRetry maps a pre-resolved resolution value onto the
// decision enum. The cmd layer validates user input before it gets
// here, so an unknown value is a wiring bug — fail loud.
func parseInstallRetry(value string) (InstallRetryDecision, error) {
	switch value {
	case "abort":
		return InstallRetryAbort, nil
	case "retry":
		return InstallRetryAgain, nil
	case "ignore":
		return InstallRetryIgnore, nil
	}
	return 0, fmt.Errorf("unknown install failure resolution %q", value)
}
