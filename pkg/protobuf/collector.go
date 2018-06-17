package protobuf

import (
	"fmt"

	"github.com/jhump/protoreflect/desc"
	"github.com/wearefair/gurl/pkg/log"
)

// Collector holds onto a cache of descriptors
type Collector struct {
	// Maps message type name to descriptor
	MessageCache map[string]*desc.MessageDescriptor
	// Maps services name to descriptor
	ServiceCache map[string]*desc.ServiceDescriptor
}

// NewCollector returns an instance of a Collector struct
func NewCollector(fileDescriptors []*desc.FileDescriptor) *Collector {
	collector := &Collector{
		MessageCache: make(map[string]*desc.MessageDescriptor),
		ServiceCache: make(map[string]*desc.ServiceDescriptor),
	}
	collector.addDescriptorsToCache(fileDescriptors)
	return collector
}

// AddDescriptors takes a slice of file descriptors, walks them, and then saves
// all message and service descriptors to a cache with the key as the FQDN
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

// ListServices lists back the services in a formatted method
func (c *Collector) ListServices() {
	serviceIndex := 1
	for name, service := range c.ServiceCache {
		fmt.Printf("%d. %s\n", serviceIndex, name)
		methods := service.GetMethods()
		for i, method := range methods {
			// TODO: This is pretty ugly and will start printing weird characters.
			fmt.Printf("\t%s. %s\n", string(toChar(i+1)), method.GetName())
		}
		serviceIndex++
	}
}

func toChar(i int) rune {
	return rune('A' - 1 + i)
}

// GetMessage takes a message descriptor's FQDN and returns the descriptor
// or an error if not found
func (c *Collector) GetMessage(name string) (*desc.MessageDescriptor, error) {
	descriptor, ok := c.MessageCache[name]
	if !ok {
		err := fmt.Errorf("No message descriptor found for %s", name)
		return nil, log.WrapError(err)
	}
	return descriptor, nil
}

// GetService takes a service descriptor's FQDN and returns the descriptor
// or an error if not found
func (c *Collector) GetService(name string) (*desc.ServiceDescriptor, error) {
	descriptor, ok := c.ServiceCache[name]
	if !ok {
		err := fmt.Errorf("No service descriptor found for %s", name)
		return nil, log.WrapError(err)
	}
	return descriptor, nil
}
