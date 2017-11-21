package grpc

import (
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

// Validator validates messages protomessages
type Validator struct {
	Factory *dynamic.MessageFactory
}

func NewValidator() *Validator {
	return &Validator{
		Factory: dynamic.NewMessageFactoryWithDefaults(),
	}
}

func (v *Validator) Validate(messageDescriptor *desc.MessageDescriptor, request string) (*dynamic.Message, error) {
	message := v.Factory.NewDynamicMessage(messageDescriptor)
	err := (&runtime.JSONPb{}).Unmarshal([]byte(request), &message)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return message, nil
}
