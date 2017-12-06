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

func NewCollector(fileDescriptors []*desc.FileDescriptor) *Collector {
	collector := &Collector{
		MessageCache: make(map[string]*desc.MessageDescriptor),
		ServiceCache: make(map[string]*desc.ServiceDescriptor),
	}
	collector.addDescriptorsToCache(fileDescriptors)
	return collector
}

func (c *Collector) AddDescriptors(fileDescriptors []*desc.FileDescriptor) {
	c.addDescriptorsToCache(fileDescriptors)
}

func (c *Collector) addDescriptorsToCache(fileDescriptors []*desc.FileDescriptor) {
	for _, descriptor := range fileDescriptors {
		messages := descriptor.GetMessageTypes()
		services := descriptor.GetServices()
		for _, message := range messages {
			c.MessageCache[message.GetFullyQualifiedName()] = message
		}
		for _, service := range services {
			c.ServiceCache[service.GetFullyQualifiedName()] = service
		}
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
		err := fmt.Errorf("No message descriptor found for %s", name)
		logger.Error(err.Error())
		return nil, err
	}
	return descriptor, nil
}

func (c *Collector) GetService(name string) (*desc.ServiceDescriptor, error) {
	descriptor, ok := c.ServiceCache[name]
	if !ok {
		err := fmt.Errorf("No service descriptor found for %s", name)
		logger.Error(err.Error())
		return nil, err
	}
	return descriptor, nil
}
