package util

import (
	"reflect"
	"testing"
)

func TestParseURI(t *testing.T) {
	testCases := []struct {
		Input    string
		Expected *URI
		Err      error
	}{
		{
			Input: "localhost:3000/fakeService.Service/fakeMethod",
			Expected: &URI{
				Host:    "localhost",
				Port:    "3000",
				Service: "fakeService.Service",
				Method:  "fakeMethod",
			},
			Err: nil,
		},
		{
			Input:    "fakeNews",
			Expected: nil,
			Err:      errInvalidURIFormat,
		},
		{
			Input:    "",
			Expected: nil,
			Err:      errInvalidURIFormat,
		},
	}

	for _, testCase := range testCases {
		uri, err := ParseURI(testCase.Input)
		if !reflect.DeepEqual(uri, testCase.Expected) {
			t.Errorf("Expected: %v\ngot: %v", testCase.Expected, uri)
		}
		if err != testCase.Err {
			t.Errorf("Expected: %v\ngot: %v", testCase.Err, err)
		}
	}
}
