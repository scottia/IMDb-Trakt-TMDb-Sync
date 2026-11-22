package imdb

import "strings"

var wafBlockedMessages = []string{
	"aws waf challenge remained after solver token and reload",
	"browser-native aws waf challenge did not clear",
}

func IsWAFBlocked(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range wafBlockedMessages {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}
