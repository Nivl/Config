package appsetup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// errNoInput signals stdin EOF before the user gave a y/n answer.
// Same sentinel pattern as internal/userinput/errNoInput.
var errNoInput = errors.New("no input; aborting")

// YesNoPrompter resolves a single yes/no question. Smaller surface than
// internal/userinput's string-returning prompts; appropriate for
// SetupGitHub's one-shot "Setup Github" question that doesn't have
// env-var pass-through semantics.
type YesNoPrompter interface {
	AskYesNo(prompt string) (bool, error)
}

// NewYesNoPrompter returns the production YesNoPrompter that reads
// from in and writes "(y/n)? " prompts + "Invalid value" feedback to
// out. Loop semantics match internal/userinput's promptYesNo helper.
func NewYesNoPrompter(in io.Reader, out io.Writer) YesNoPrompter {
	return &realYesNoPrompter{
		in:  bufio.NewReader(in),
		out: out,
	}
}

// realYesNoPrompter is the production impl. Unexported so consumers
// always go through the interface.
type realYesNoPrompter struct {
	in  *bufio.Reader
	out io.Writer
}

// AskYesNo loops on "<prompt> (y/n)? "; first-byte dispatch
// (y/Y → true, n/N → false, anything else "Invalid value" + retry).
// EOF returns errNoInput.
func (p *realYesNoPrompter) AskYesNo(prompt string) (bool, error) {
	for {
		if _, err := fmt.Fprintf(p.out, "%s (y/n)? ", prompt); err != nil {
			return false, fmt.Errorf("write prompt: %w", err)
		}
		line, err := p.in.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && line == "" {
				return false, errNoInput
			}
			if !errors.Is(err, io.EOF) {
				return false, fmt.Errorf("read input: %w", err)
			}
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if _, ferr := fmt.Fprintln(p.out, "Invalid value"); ferr != nil {
				return false, fmt.Errorf("write retry hint: %w", ferr)
			}
			continue
		}
		switch line[0] {
		case 'y', 'Y':
			return true, nil
		case 'n', 'N':
			return false, nil
		}
		if _, ferr := fmt.Fprintln(p.out, "Invalid value"); ferr != nil {
			return false, fmt.Errorf("write retry hint: %w", ferr)
		}
	}
}
