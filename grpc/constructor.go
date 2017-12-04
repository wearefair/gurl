package grpc

import (
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

// Constructor wraps construction of dynamic protomessages
type Constructor struct {
	Factory *dynamic.MessageFactory
}

func NewConstructor() *Constructor {
	constructor := &Constructor{
		Factory: dynamic.NewMessageFactoryWithDefaults(),
	}
	constructor.overrideKnownTypes()
	return constructor
}

func (c *Constructor) Construct(messageDescriptor *desc.MessageDescriptor, request string) (*dynamic.Message, error) {
	// Find way to either edit this to use the timestamp.Timestamp{} type
	message := c.Factory.NewDynamicMessage(messageDescriptor)
	err := (&runtime.JSONPb{}).Unmarshal([]byte(request), message)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return message, nil
}

// Override certain known types
func (c *Constructor) overrideKnownTypes() {
	registry := c.Factory.GetKnownTypeRegistry()
	messageDescriptor, err := desc.LoadMessageDescriptorForMessage(&timestamp.Timestamp{})
	if err != nil {
		logger.Error(err.Error())
		return
	}
	message := dynamic.NewMessage(messageDescriptor)
	registry.AddKnownType(message)
}

func (c *Constructor) RegisterFileDescriptors(fileDescriptors []*desc.FileDescriptor) {
	for _, fileDescriptor := range fileDescriptors {
		messageDescriptors := fileDescriptor.GetMessageTypes()
		for _, messageDescriptor := range messageDescriptors {
			message := dynamic.NewMessage(messageDescriptor)
			c.Factory.GetKnownTypeRegistry().AddKnownType(message)
		}
	}
}
