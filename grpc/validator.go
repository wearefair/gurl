package grpc

import (
	"encoding/json"
	"fmt"

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
		field := messageDescriptor.FindFieldByName(key)
		if field == nil {
			err := fmt.Errorf("Field %s not found on message %s", key, messageDescriptor.GetName())
			logger.Error(err.Error())
			return nil, err
		}
		message.SetFieldByName(key, val)
	}
	return message, nil
}
