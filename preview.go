package main

import "regexp"

const previewPrefix = "here's your preview : "

type previewRule struct {
	source      *regexp.Regexp
	replacement string
}

// Go's regexp has no lookahead, so the character after the domain is captured
// and written back through ${1}.
var previewRules = []previewRule{
	{
		source:      regexp.MustCompile(`(?i)https?://(?:www\.)?instagram\.com([/?#]|$)`),
		replacement: "https://oginstagram.com${1}",
	},
	{
		source:      regexp.MustCompile(`(?i)https?://(?:www\.)?(?:x\.com|twitter\.com)([/?#]|$)`),
		replacement: "https://www.fixupx.com${1}",
	},
}

// createPreview rewrites supported links. It reports false if there are none.
func createPreview(text string) (string, bool) {
	found := false
	for _, rule := range previewRules {
		if rule.source.MatchString(text) {
			found = true
			text = rule.source.ReplaceAllString(text, rule.replacement)
		}
	}
	if !found {
		return "", false
	}
	return previewPrefix + text, true
}
