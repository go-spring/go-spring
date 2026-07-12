namespace go echo

struct EchoRequest {
    1: required string message
}

struct EchoResponse {
    1: required string message
}

service EchoService {
    EchoResponse Echo(1: EchoRequest req)
}
