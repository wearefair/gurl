package grpc

import (
	"os"
	"path/filepath"
	"strings"

	set "gopkg.in/fatih/set.v0"

	"github.com/davecgh/go-spew/spew"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/wearefair/gurl/log"
)

var (
	logger = log.Logger()
)

// ProtoWalker walks directories and collects proto file descriptors
type ProtoWalker struct {
	Descriptors []*desc.FileDescriptor
	Paths       *set.Set
}

// NewProtoWalker creates instance of ProtoWalker
func NewProtoWalker() *ProtoWalker {
	return &ProtoWalker{Paths: set.New()}
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
				p.Paths.Add(pathSplit[0])
			} else {
				p.Paths.Add(pathSplit[1])
			}
		}
		return nil
	})
}

// Collect picks up and parses proto paths
func (p *ProtoWalker) Collect(trees []string) error {
	for _, tree := range trees {
		p.walkDirs(tree)
	}
	parser := protoparse.Parser{
		ImportPaths: trees,
	}
	fileDescriptors, err := parser.ParseFiles(set.StringSlice(p.Paths)...)
	if err != nil {
		logger.Error(err.Error())
		return err
	}
	p.Descriptors = fileDescriptors
	return nil
}

func (p *ProtoWalker) ListMessages() {
	messages := []*desc.MessageDescriptor{}
	for _, descriptor := range p.Descriptors {
		messages = append(messages, descriptor.GetMessageTypes()...)
	}
	spew.Dump(messages)
}

func (p *ProtoWalker) ListServices() {
	services := []*desc.ServiceDescriptor{}
	for _, descriptor := range p.Descriptors {
		services = append(services, descriptor.GetServices()...)
	}
	spew.Dump(services)
}
