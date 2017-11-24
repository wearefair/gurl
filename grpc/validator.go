package grpc

import (
	"encoding/json"

	"github.com/davecgh/go-spew/spew"
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
	messageMap := map[string]interface{}{}
	err := json.Unmarshal([]byte(request), &messageMap)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	for key, val := range messageMap {
		spew.Dump(val)
		field := messageDescriptor.FindFieldByName(key)
		spew.Dump(field.AsFieldDescriptorProto())
		intVal := val.(int)
		message.SetFieldByName(key, intVal)
	}
	spew.Dump(message)
	err = (&runtime.JSONPb{}).Unmarshal([]byte(request), &message)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return message, nil
}
