package grpc

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"

	set "gopkg.in/fatih/set.v0"
)

func Construct(messageDescriptor *desc.MessageDescriptor, request string) (*dynamic.Message, error) {
	message := dynamic.NewMessage(messageDescriptor)
	err := (&runtime.JSONPb{}).Unmarshal([]byte(request), message)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return message, nil
}

// Parses protos and returns the correct protos
func Collect(importPaths, servicePaths []string) ([]*desc.FileDescriptor, error) {
	concat := append(importPaths, servicePaths...)
	paths := set.New()
	for _, path := range servicePaths {
		paths = walkDirs(path, paths)
	}
	parser := protoparse.Parser{ImportPaths: concat}
	descriptors, err := parser.ParseFiles(set.StringSlice(paths)...)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return descriptors, nil
}

func walkDirs(tree string, paths *set.Set) *set.Set {
	filepath.Walk(tree, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(info.Name()) == ".proto" {
			// Need to just add the path after the directory that things are pointed to
			pathSplit := strings.SplitAfter(path, tree+"/")
			// This is not going to end well - add conditional logic around it if path is '.'
			if len(pathSplit) < 2 {
				paths.Add(pathSplit[0])
			} else {
				paths.Add(pathSplit[1])
			}
		}
		return nil
	})
	return paths
}
