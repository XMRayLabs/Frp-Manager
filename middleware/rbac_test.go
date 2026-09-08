package middleware

import "testing"

func TestRuleMatchedRejectsInvalidRegexWithoutPanic(t *testing.T) {
	if ruleMatched(ruleMatchParam{
		RuleMethod:    "GET",
		RulePath:      "[",
		RequestPath:   "/api/v1/client/list",
		RequestMethod: "GET",
	}) {
		t.Fatal("invalid regular expression must not match")
	}
}
