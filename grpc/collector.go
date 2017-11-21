package grpc

import (
	"fmt"

	"github.com/jhump/protoreflect/desc"
)

// Collector holds onto a cache of descriptors
type Collector struct {
	// Maps message type name to descriptor
	MessageCache map[string]*desc.MessageDescriptor
	// Maps services name to descriptor
	ServiceCache map[string]*desc.ServiceDescriptor
}

// Instantiates MessageCollector from file descriptors
func NewCollector(fileDescriptors []*desc.FileDescriptor) *Collector {
	messageCache := map[string]*desc.MessageDescriptor{}
	serviceCache := map[string]*desc.ServiceDescriptor{}
	for _, descriptor := range fileDescriptors {
		messages := descriptor.GetMessageTypes()
		services := descriptor.GetServices()
		for _, message := range messages {
			messageCache[message.GetFullyQualifiedName()] = message
		}
		for _, service := range services {
			serviceCache[service.GetFullyQualifiedName()] = service
		}
	}
	return &Collector{
		MessageCache: messageCache,
		ServiceCache: serviceCache,
	}
}

func (c *Collector) ListServices() {
	serviceIndex := 1
	for name, service := range c.ServiceCache {
		fmt.Printf("%d. %s\n", serviceIndex, name)
		methods := service.GetMethods()
		for i, method := range methods {
			fmt.Printf("\t%s. %s\n", string(toChar(i+1)), method.GetName())
		}
		serviceIndex++
	}
}

func toChar(i int) rune {
	return rune('A' - 1 + i)
}

func (c *Collector) GetMessage(name string) (*desc.MessageDescriptor, error) {
	descriptor, ok := c.MessageCache[name]
	if !ok {
		return nil, fmt.Errorf("No message descriptor found for %s", name)
	}
	return descriptor, nil
}

func (c *Collector) GetService(name string) (*desc.ServiceDescriptor, error) {
	descriptor, ok := c.ServiceCache[name]
	if !ok {
		return nil, fmt.Errorf("No service descriptor found for %s", name)
	}
	return descriptor, nil
}
