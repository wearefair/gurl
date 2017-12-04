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
	importPathDescriptors  []*desc.FileDescriptor
	servicePathDescriptors []*desc.FileDescriptor
	importPaths            *set.Set
	servicePaths           *set.Set
}

// NewProtoWalker creates instance of ProtoWalker
func NewProtoWalker() *ProtoWalker {
	return &ProtoWalker{importPaths: set.New(), servicePaths: set.New()}
}

// Walk all directories and collects dirs where protos are found. This needs to be done
// because otherwise the proto parser has no idea where to look for related protobufs
func (p *ProtoWalker) walkDirs(tree string, pathKeeper *set.Set) {
	filepath.Walk(tree, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(info.Name()) == ".proto" {
			// Need to just add the path after the directory that things are pointed to
			pathSplit := strings.SplitAfter(path, tree+"/")
			// This is not going to end well - add conditional logic around it if path is '.'
			if len(pathSplit) < 2 {
				pathKeeper.Add(pathSplit[0])
			} else {
				pathKeeper.Add(pathSplit[1])
			}
		}
		return nil
	})
}

// Collect picks up and parses proto paths
func (p *ProtoWalker) Collect(importPaths, servicePaths []string) error {
	for _, path := range importPaths {
		p.walkDirs(path, p.importPaths)
	}
	for _, path := range servicePaths {
		p.walkDirs(path, p.servicePaths)
	}
	importParser := protoparse.Parser{
		ImportPaths: importPaths,
	}
	serviceParser := protoparse.Parser{
		ImportPaths: servicePaths,
	}
	importDescriptors, err := importParser.ParseFiles(set.StringSlice(p.importPaths)...)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	p.importPathDescriptors = importDescriptors
	serviceDescriptors, err := serviceParser.ParseFiles(set.StringSlice(p.servicePaths)...)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	p.servicePathDescriptors = serviceDescriptors
	return nil
}

func (p *ProtoWalker) GetImportFileDescriptors() []*desc.FileDescriptor {
	return p.importPathDescriptors
}

func (p *ProtoWalker) GetServiceFileDescriptors() []*desc.FileDescriptor {
	return p.servicePathDescriptors
}
