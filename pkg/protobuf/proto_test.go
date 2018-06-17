package protobuf

import (
	"path/filepath"
	"testing"
)

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

// Tests the construction of a message descriptor using the
// helloworld.proto found in the test folder. This is not
// really an amazing test, but at the very least validates that
// a dynamic message can be constructed from a JSON string with
// the correct fields.
func TestConstruct(t *testing.T) {
	// Happy path - we get a valid message descriptor and we're able
	// to unmarshal a JSON message onto it.
	descriptors, err := Collect([]string{}, absolutePathify([]string{"./test/"}))
	if err != nil {
		t.Errorf("Error collecting test descriptors: %s", err.Error())
	}
	collector := NewCollector(descriptors)
	// Get service descriptor for helloworld package
	serviceDescriptor, err := collector.GetService("helloworld.Greeter")
	if err != nil {
		t.Errorf("Error getting service descriptors: %s", err.Error())
	}
	// Get RPC
	methodDescriptor := serviceDescriptor.FindMethodByName("SayHello")
	if methodDescriptor == nil {
		t.Error("Error getting method descriptor")
	}
	methodProto := methodDescriptor.AsMethodDescriptorProto()
	messageDescriptor, err := collector.GetMessage(
		NormalizeMessageName(*methodProto.InputType),
	)
	if err != nil {
		t.Errorf("Error getting message for method %s", err.Error())
	}

	// Making an anonymous message struct to convert into a JSON string.
	messageStr := `{ "name": "cat" }`

	// Actual construction test here now that we have a real message descriptor.
	constructed, err := Construct(messageDescriptor, messageStr)
	if err != nil {
		t.Errorf("Error constructing message %s", err.Error())
	}
	val := constructed.GetFieldByName("name")
	if val != "cat" {
		t.Errorf("Expected field name: cat, got: %s", val)
	}

	// Unhappy path - We attempt to marshal a message as JSON string with invalid
	// field names onto the message. This will construct a message with empty strings.
	invalidMessageStr := `{ "fake": "news" }`
	invalidMessage, err := Construct(messageDescriptor, invalidMessageStr)
	if err != nil {
		t.Errorf("Error constructing message %s", err.Error())
	}
	invalidVal := invalidMessage.GetFieldByName("name")
	if invalidVal != "" {
		t.Errorf("Expected value to be empty, got %s", invalidVal)
	}
}

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

// Helper to get an absolute path
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
