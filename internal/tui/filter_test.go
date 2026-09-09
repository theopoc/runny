package tui

import "testing"

func TestTargetFilterRegexUsesGoRegexpSemantics(t *testing.T) {
	filter := parseTargetFilter(`re:^services/(api|web)$`)
	if filter.err != nil {
		t.Fatalf("parse regex filter: %v", filter.err)
	}
	for _, value := range []string{"services/api", "services/web"} {
		if !filter.matches(value) {
			t.Fatalf("regex filter should match %q", value)
		}
	}
	for _, value := range []string{"services/worker", "prefix/services/api"} {
		if filter.matches(value) {
			t.Fatalf("anchored regex filter should not match %q", value)
		}
	}
}

func TestTargetFilterRegexIsCaseSensitiveUnlessPatternOptsIn(t *testing.T) {
	if parseTargetFilter(`re:^API$`).matches("api") {
		t.Fatal("regex filter should be case-sensitive by default")
	}
	if !parseTargetFilter(`re:(?i)^API$`).matches("api") {
		t.Fatal("regex filter should support Go regexp inline case-insensitive mode")
	}
}

func TestTargetFilterRegexReportsInvalidPattern(t *testing.T) {
	filter := parseTargetFilter(`re:[`)
	if filter.err == nil {
		t.Fatal("invalid regex filter should report a compile error")
	}
	if filter.matches("api") {
		t.Fatal("invalid regex filter should match no target")
	}
}

func TestTargetFilterEmptyRegexIsInactive(t *testing.T) {
	filter := parseTargetFilter("re:")
	if filter.err != nil {
		t.Fatalf("empty regex filter: %v", filter.err)
	}
	if filter.active() {
		t.Fatal("empty regex filter should behave like an empty target filter")
	}
}

func TestTargetFilterExactPrefixRemainsActive(t *testing.T) {
	filter := parseTargetFilter("'")
	if !filter.active() || !filter.matches("api") {
		t.Fatal("exact prefix alone should preserve existing active-filter behavior")
	}
}

func TestTargetFilterRegexReturnsEveryNonOverlappingMatch(t *testing.T) {
	ranges := parseTargetFilter(`re:api`).matchRanges("api-api")
	want := []filterMatchRange{{start: 0, end: 3}, {start: 4, end: 7}}
	if len(ranges) != len(want) {
		t.Fatalf("match ranges = %#v, want %#v", ranges, want)
	}
	for i := range want {
		if ranges[i] != want[i] {
			t.Fatalf("match range %d = %#v, want %#v", i, ranges[i], want[i])
		}
	}
}

func TestTargetFilterZeroWidthRegexMatchesWithoutHighlightRange(t *testing.T) {
	filter := parseTargetFilter(`re:^`)
	if !filter.matches("api") {
		t.Fatal("zero-width regex should still match target path")
	}
	if ranges := filter.matchRanges("api"); len(ranges) != 0 {
		t.Fatalf("zero-width regex highlight ranges = %#v, want none", ranges)
	}
}

func TestSharedFilterMatcherDoesNotEnableTargetRegexSyntax(t *testing.T) {
	if filterMatches("run", `re:^run$`) {
		t.Fatal("shared palette/history matcher should not interpret target regex syntax")
	}
}
