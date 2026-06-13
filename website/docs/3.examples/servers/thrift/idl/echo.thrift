namespace go proto

struct EchoRequest {
1: required string message
}

struct EchoResponse {
1: required string message
}

service EchoService {
  EchoResponse echo(1: EchoRequest req)
}
