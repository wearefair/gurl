package grpc

import (
	"path/filepath"
	"testing"
)

func TestCollect(t *testing.T) {
	type expectedProtoInfo struct {
		Name     string
		Package  string
		IsProto3 bool
	}
	testCases := []struct {
		ImportPaths  []string
		ServicePaths []string
		Expected     []expectedProtoInfo
		Err          error
	}{
		// Shallow check to make sure we collect the hello world proto
		{
			ImportPaths:  []string{},
			ServicePaths: absolutePathify([]string{"./test/"}),
			Expected: []expectedProtoInfo{
				expectedProtoInfo{
					Name:     "helloworld.proto",
					Package:  "helloworld",
					IsProto3: true,
				},
			},
			Err: nil,
		},
		// Check to make sure that nothing throws when import paths are empty
		{
			ImportPaths:  []string{},
			ServicePaths: []string{},
			Expected:     nil,
			Err:          nil,
		},
	}
	for _, testCase := range testCases {
		results, err := Collect(testCase.ImportPaths, testCase.ServicePaths)
		if err != testCase.Err {
			t.Errorf("Expected error: %v\ngot: %v", testCase.Err, err)
		}
		if results != nil {
			for i, result := range results {
				if result.GetName() != testCase.Expected[i].Name {
					t.Errorf("Expected name %s, got %s", testCase.Expected[i].Name, result.GetName())
				}
				if result.GetPackage() != testCase.Expected[i].Package {
					t.Errorf("Expected package %s, got %s", testCase.Expected[i].Package, result.GetPackage())
				}
				if result.IsProto3() != testCase.Expected[i].IsProto3 {
					t.Errorf("Expected to proto3 result to be %t, got %t", testCase.Expected[i].IsProto3, result.IsProto3())
				}
			}
		}
	}
}

func absolutePathify(paths []string) []string {
	absolute := make([]string, len(paths))
	for i, path := range paths {
		abs, err := filepath.Abs(path)
		if err != nil {
			panic(err)
		}
		absolute[i] = abs
	}
	return absolute
}
