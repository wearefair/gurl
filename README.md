## GRPC Curl

A tool for curling gRPC services

### Setup
Configure gurl with the *absolute path* to your protos, so it can load them.

```
gurl config
```

### Example
Request format:
```
gurl -u k8://k8-context/localhost:50051/service.name.here/method.name.here -d '{ "field name": "field value" }'
```

### Reference for JSON Types
You should format JSON according to the protobuf docs laid out [here](https://developers.google.com/protocol-buffers/docs/proto3#json).
