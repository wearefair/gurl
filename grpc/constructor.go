package grpc

import (
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
	return constructor
}

func (c *Constructor) Construct(messageDescriptor *desc.MessageDescriptor, request string) (*dynamic.Message, error) {
	message := c.Factory.NewDynamicMessage(messageDescriptor)
	err := (&runtime.JSONPb{}).Unmarshal([]byte(request), message)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return message, nil
}
