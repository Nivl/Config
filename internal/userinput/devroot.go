package userinput

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
)

// DevRoot resolves DEV_ROOT via the pre-resolved-value fast-path: when
// prearg is non-empty, return verbatim. The cmd layer is responsible
// for sourcing it from --dev-root / DEV_ROOT. Otherwise prompt
// `Where do you want your dev root to be? [<default>]: ` (default is
// homeDir/dev); empty input accepts the default. NOT persisted; the
// value is consumed by configgen.SetupZshrc and baked into ~/.zshrc.
func DevRoot(prearg string, in io.Reader, out io.Writer, homeDir string) (string, error) {
	if prearg != "" {
		return prearg, nil
	}

	defaultPath := filepath.Join(homeDir, "dev")
	if _, err := fmt.Fprintf(out, "Where do you want your dev root to be? [%s]: ", defaultPath); err != nil {
		return "", fmt.Errorf("write prompt: %w", err)
	}
	r := bufio.NewReader(in)
	line, err := readLine(r)
	if err != nil {
		return "", fmt.Errorf("read line: %w", err)
	}
	if line == "" {
		return defaultPath, nil
	}
	return line, nil
}
