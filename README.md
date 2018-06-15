## GRPC Curl
A tool for curling gRPC services.

### Why?
There aren't many tools out there that make debugging gRPC easy. One of the approaches is to create a client stub for your language of choice. However, this requires a compilation step that adds a lot of overhead to your workflow.

When debugging a REST API, the instinct is often to use curl. gurl is meant to facilitate a similar workflow for debugging gRPC services, since it only requires a path to where your protos are located. No compilation or stub generation required! Just configure gurl to point to your protos, start your server, form your request as a JSON string (as you would for curl), and go.

### Requirements & Installing
gurl is built in Go 1.10.

gurl uses dep for dependencies. However, vendored deps are not checked in because of the presence of the Gopkg.lock. To build locally:
```bash
# Install dep, or use brew install dep on OS X
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

# Pulls all dependencies
make deps

# Create a cross platform build
make build
```

You can also download a copy of gurl on the platform of your choice from gurl's [releases](https://github.com/wearefair/gurl/releases) page.

### Setup
Configure gurl with the *absolute path* to your protos, so it can load them.

```bash
gurl config
```

gurl will prompt for your import paths and service paths (the nomenclature for these paths is poor and prone to change.) Both are just a comma delimited list of absolute paths to your proto files.

### Request Format
gurl's request format is as follows:
```bash
gurl -u <protocol|optional>://<k8-context|optional>/<host|kubernetes-service-name>:<port>/<service>/<rpc> -d '{ "field_name": "field_value" }'
```

gurl also supports forwarding requests to a Kubernetes server, so long as your kubeconfig is located in the default director of $HOME/.kube/config. If you format your request with the protocol k8://, gurl will know to send the Kubernetes request to a Kubernetes service via port-forwarding.

Valid URL formats:
```bash
# URL with protocol
http://localhost:50051/helloworld/Greeter -d '{ "name": "cat cai" }'

# URL without protocol
localhost:50051/helloworld/Greeter -d '{ "name": "cat cai" }'

# URL with K8 protocol - in this case, my-service should be your Kubernetes service name, along with the service port used to expose your gRPC service
k8://my-k8-context/my-service:50051/helloworld/Greeter -d '{ "name": "cat cai" }'
```

### Reference for JSON Types
You should format JSON according to the protobuf docs laid out [here](https://developers.google.com/protocol-buffers/docs/proto3#json).

### Special Thanks
Huge props to [jhump](https://github.com/jhump) for his [protoreflect](https://github.com/jhump/protoreflect) package, which gurl makes heavy use of.
