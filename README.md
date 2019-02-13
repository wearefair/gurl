## gURL (gRPC cURL)
A tool for cURLing gRPC services.

### Why?
There aren't many tools out there that make debugging gRPC easy. One of the approaches is to create a client stub for your language of choice. However, this requires a compilation step that adds a lot of overhead to your workflow.

When debugging a REST API, the instinct is often to use cURL. gURL is meant to facilitate a similar workflow for debugging gRPC services, since it only requires a path to where your protos are located. No compilation or stub generation required! Just configure gURL to point to your protos, start your server, form your request as a JSON string (as you would for cURL), and go.

### Requirements & Installing
gURL is built in Go 1.10.

gURL uses dep for dependencies. However, vendored deps are not checked in because of the presence of the Gopkg.lock. To build locally:
```bash
# Install dep, or use brew install dep on OS X
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

# Pulls all dependencies
make deps

# Create a cross platform build
make build
```

This will generate cross platform binaries in the `release/` folder. You'll need to give yourself executable permissions for the relevant binary and then move it somewhere in your $PATH.

Example:
```bash
chmod +x gurl-darwin-amd64
mv gurl-darwin-amd64 /usr/local/bin/gurl
```

You can also download a copy of gURL on the platform of your choice from gURL's [releases](https://github.com/wearefair/gurl/releases) page.

### Setup
Configure gURL with the *absolute path* to your protos, so it can load them.

```bash
gurl config
```

gURL will prompt for your import paths and service paths (the nomenclature for these paths is poor and prone to change.) Both are just a comma delimited list of absolute paths to your proto files. The distinction is that your import paths are external protos that you're importing, and your service paths are your protos.

Here is an example of a gURL config (found at $HOME/.gurl/config):

```yaml
local:
  importpaths:
  - ""
  servicepaths:
  - /Users/johnsmith/go/src/github.com/johnsmith/myprotos
kubeconfig: /Users/johnsmith/.kube/config
```

### Request Format
gURL's request format is as follows:
```bash
gurl -u <protocol|optional>://<k8-context|optional>/<host|kubernetes-service-name>:<port>/<service>/<rpc> -d '{ "field_name": "field_value" }'
```

gURL also supports forwarding requests to a Kubernetes server, so long as your kubeconfig is located in the default director of $HOME/.kube/config. If you format your request with the protocol k8://, gURL will know to send the Kubernetes request to a Kubernetes service via port-forwarding.

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

### Caveats/Places to Improve
Caveats:
- This only supports unary calls

Places to improve:
- Configurable log levels
- gURL configurations that are local to a project
- Pass in proto files as an arg instead of as a configuration

### Contributing to gURL
There's a lot of potential work to be done for gURL and we welcome contributions.

#### How to Contribute
If you're interested in contributing, first look through gURL's [issues](https://github.com/wearefair/gurl/issues) and [pull requests](https://github.com/wearefair/gurl/pulls) for anything that might be similar (there's no reason to unnecessarily duplicate work).

If you don't see anything listed, feel free to open an issue or pull request to open up a discussion.

### Special Thanks
Huge props to [jhump](https://github.com/jhump) for his [protoreflect](https://github.com/jhump/protoreflect) package, which gURL makes heavy use of.
