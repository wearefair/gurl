package proxy

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/metadata"
)

func TestMergeHttpHeadersToMetadata(t *testing.T) {
	testCases := []struct {
		Md       metadata.MD
		Headers  http.Header
		Expected metadata.MD
	}{
		// Empty metadata merges with HTTP headers
		{
			Md:       metadata.MD{},
			Headers:  map[string][]string{"foo": []string{"bar"}},
			Expected: metadata.Pairs("foo", "bar"),
		},
		// Headers with multiple values merge properly
		{
			Md:       metadata.MD{},
			Headers:  map[string][]string{"hello": []string{"world", "boo"}},
			Expected: metadata.Pairs("hello", "world", "hello", "boo"),
		},
		// Original metadata is not touched if headers are empty
		{
			Md:       metadata.Pairs("foo", "bar"),
			Expected: metadata.Pairs("foo", "bar"),
		},
	}

	for i, testCase := range testCases {
		res := mergeHttpHeadersToMetadata(testCase.Md, testCase.Headers)

		if !cmp.Equal(res, testCase.Expected) {
			t.Errorf("[%d] - Expected: %+v\nActual: %+v\n", i, testCase.Expected, res)
		}
	}
}
