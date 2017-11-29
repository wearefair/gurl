package grpc

import (
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	//"github.com/catherinetcai/grpc-gateway/runtime"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

// Constructor wraps construction of dynamic protomessages
type Constructor struct {
	Factory *dynamic.MessageFactory
}

func NewConstructor() *Constructor {
	return &Constructor{
		Factory: dynamic.NewMessageFactoryWithDefaults(),
	}
}

func (v *Constructor) Construct(messageDescriptor *desc.MessageDescriptor, request string) (*dynamic.Message, error) {
	message := v.Factory.NewDynamicMessage(messageDescriptor)
	//	messageMap := map[string]interface{}{}
	//	err := json.Unmarshal([]byte(request), &messageMap)
	//	if err != nil {
	//		logger.Error(err.Error())
	//		return nil, err
	//	}
	err := (&runtime.JSONPb{}).Unmarshal([]byte(request), message)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}
	return message, nil
}
