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
			Input: "k8://sandbox-general/public-api:80/fakeService.Service/fakeMethod",
			Expected: &URI{
				Protocol: "k8",
				Context:  "sandbox-general",
				Service:  "public-api",
				Port:     "80",
				RPC:      "fakeService.Service",
				Method:   "fakeMethod",
			},
			Err: nil,
		},
		{
			Input: "localhost:3000/fakeService.Service/fakeMethod",
			Expected: &URI{
				Service: "localhost",
				Port:    "3000",
				RPC:     "fakeService.Service",
				Method:  "fakeMethod",
			},
			Err: nil,
		},
		{
			Input: "http://localhost:3000/fakeService.Service/fakeMethod",
			Expected: &URI{
				Protocol: "http",
				Service:  "localhost",
				Port:     "3000",
				RPC:      "fakeService.Service",
				Method:   "fakeMethod",
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
