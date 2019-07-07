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
		// Parse K8 protocol with context set
		{
			Input: "k8://sandbox-general/public-api:80/fakeService.Service/fakeRPC",
			Expected: &URI{
				Protocol: "k8",
				Context:  "sandbox-general",
				Host:     "public-api",
				Port:     "80",
				Service:  "fakeService.Service",
				RPC:      "fakeRPC",
			},
			Err: nil,
		},
		// Parse K8 protocol without context set
		{
			Input: "k8://public-api:80/fakeService.Service/fakeRPC",
			Expected: &URI{
				Protocol: "k8",
				Host:     "public-api",
				Port:     "80",
				Service:  "fakeService.Service",
				RPC:      "fakeRPC",
			},
			Err: nil,
		},
		// Parse input without protocol set
		{
			Input: "localhost:3000/fakeService.Service/fakeRPC",
			Expected: &URI{
				Host:    "localhost",
				Port:    "3000",
				Service: "fakeService.Service",
				RPC:     "fakeRPC",
			},
			Err: nil,
		},
		// Parse input with protocol set
		{
			Input: "http://localhost:3000/fakeService.Service/fakeRPC",
			Expected: &URI{
				Protocol: "http",
				Host:     "localhost",
				Port:     "3000",
				Service:  "fakeService.Service",
				RPC:      "fakeRPC",
			},
			Err: nil,
		},
		// Parse input without service or RPC
		{
			Input: "k8://fake-context/k8-service:3000/",
			Expected: &URI{
				Protocol: "k8",
				Host:     "k8-service",
				Port:     "3000",
				Context:  "fake-context",
			},
			Err: nil,
		},
		// Input that's a completely hot garbage returns error
		{
			Input:    "fakeNews",
			Expected: nil,
			Err:      errInvalidURIFormat,
		},
		// Empty input returns error
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
