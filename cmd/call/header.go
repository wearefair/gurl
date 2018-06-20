package call

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/metadata"
)

type flagMetadata metadata.MD

func (f flagMetadata) String() string {
	builder := &strings.Builder{}
	for key, vals := range f {
		builder.WriteString(fmt.Sprintf("%s: %s; ", key, strings.Join(vals, ",")))
	}
	return builder.String()
}

func (f flagMetadata) Set(val string) error {
	components := strings.Split(val, ":")
	if len(components) != 2 {
		return fmt.Errorf("Header must be in the format '<Header-Name>:<Header-Value>'")
	}
	metadata.MD(f).Append(components[0], components[1])
	return nil
}

func (f flagMetadata) Type() string {
	return "metadata.MD"
}
