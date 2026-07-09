package operation_setting

import "strings"

var DemoSiteEnabled = false
var SelfUseModeEnabled = false

var AutomaticDisableKeywords = []string{
	"Your credit balance is too low",
	"This organization has been disabled.",
	"You exceeded your current quota",
	"Permission denied",
	"The security token included in the request is invalid",
	"Operation not allowed",
	"Your account is not authorized",
}

var AutomaticDisableIgnoreKeywords = []string{}

func AutomaticDisableKeywordsToString() string {
	return strings.Join(AutomaticDisableKeywords, "\n")
}

func AutomaticDisableKeywordsFromString(s string) {
	AutomaticDisableKeywords = parseAutomaticDisableKeywordLines(s)
}

func AutomaticDisableIgnoreKeywordsToString() string {
	return strings.Join(AutomaticDisableIgnoreKeywords, "\n")
}

func AutomaticDisableIgnoreKeywordsFromString(s string) {
	AutomaticDisableIgnoreKeywords = parseAutomaticDisableKeywordLines(s)
}

func parseAutomaticDisableKeywordLines(s string) []string {
	keywords := []string{}
	for _, k := range strings.Split(s, "\n") {
		k = strings.TrimSpace(k)
		k = strings.ToLower(k)
		if k != "" {
			keywords = append(keywords, k)
		}
	}
	return keywords
}
