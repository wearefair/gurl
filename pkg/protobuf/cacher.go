package protobuf

import "github.com/jhump/protoreflect/desc"

// Cacher is the public interface for registering FileDescriptors and getting back
// Message and ServiceDescriptors.
type Cacher interface {
	// AddDescriptors takes in a slice of file descriptors, walks them, and collects
	// all message/service descriptors so that they can be returned in the GetMessage
	// and GetService methods.
	AddDescriptors(fileDescriptors []*desc.FileDescriptor)
	// GetMessage takes a message descriptor's FQDN and returns the descriptor
	GetMessage(fqdn string) (*desc.MessageDescriptor, error)
	// GetService takes a service descriptor's FQDN and returns the descriptor
	GetService(fqdn string) (*desc.ServiceDescriptor, error)
}
