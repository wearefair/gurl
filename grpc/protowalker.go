package grpc

import (
	"os"
	"path/filepath"
	"strings"

	set "gopkg.in/fatih/set.v0"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/wearefair/gurl/log"
)

var (
	logger = log.Logger()
)

// ProtoWalker walks directories and collects proto file descriptors
type ProtoWalker struct {
	fileDescriptors []*desc.FileDescriptor
	paths           *set.Set
}

// NewProtoWalker creates instance of ProtoWalker
func NewProtoWalker() *ProtoWalker {
	return &ProtoWalker{paths: set.New()}
}

// Walk all directories and collects dirs where protos are found. This needs to be done
// because otherwise the proto parser has no idea where to look for related protobufs
func (p *ProtoWalker) walkDirs(tree string) {
	filepath.Walk(tree, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(info.Name()) == ".proto" {
			// Need to just add the path after the directory that things are pointed to
			pathSplit := strings.SplitAfter(path, tree+"/")
			// This is not going to end well - add conditional logic around it if path is '.'
			if len(pathSplit) < 2 {
				p.paths.Add(pathSplit[0])
			} else {
				p.paths.Add(pathSplit[1])
			}
		}
		return nil
	})
}

// Collect picks up and parses proto paths
func (p *ProtoWalker) Collect(importPaths, servicePaths []string) error {
	concat := append(importPaths, servicePaths...)
	for _, path := range servicePaths {
		p.walkDirs(path)
	}
	parser := protoparse.Parser{
		ImportPaths: concat,
	}
	descriptors, err := parser.ParseFiles(set.StringSlice(p.paths)...)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	p.fileDescriptors = descriptors
	return nil
}

func (p *ProtoWalker) GetFileDescriptors() []*desc.FileDescriptor {
	return p.fileDescriptors
}
