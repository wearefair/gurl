package util

import "testing"

func TestNormalizeMessageName(t *testing.T) {
	testCases := []struct {
		Input    string
		Expected string
	}{
		{
			Input:    ".fakeMessage",
			Expected: "fakeMessage",
		},
		{
			Input:    "anotherMessage",
			Expected: "anotherMessage",
		},
		{
			Input:    "",
			Expected: "",
		},
	}

	for _, testCase := range testCases {
		normalized := NormalizeMessageName(testCase.Input)
		if normalized != testCase.Expected {
			t.Errorf("Expected: %s, got: %s", testCase.Expected, normalized)
		}
	}
}
