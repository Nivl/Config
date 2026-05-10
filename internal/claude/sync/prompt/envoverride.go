package prompt

// parseMergeResolution maps a CLAUDE_MERGE_RESOLUTION value (whether
// sourced from the --merge-resolution flag, the env var, or a future
// caller) to a Choice. Returns (choice, true) for the three canonical
// values; (zero, false) for empty / unknown (caller falls through to
// the decisions cache).
func parseMergeResolution(v string) (Choice, bool) {
	switch v {
	case "keep-local":
		return ChoiceKeepLocal, true
	case "take-remote":
		return ChoiceTakeRemote, true
	case "skip":
		return ChoiceSkip, true
	default:
		return 0, false
	}
}
