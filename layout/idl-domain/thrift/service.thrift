// service.thrift — Apache Thrift IDL for the GS_PROJECT_NAME service.
// Generate with: thriftgo -g go -o gen service.thrift

namespace go thrift

// Ping
struct PingReq {
    1: string name
}

struct PingResp {
    1: string message
}

// Order
struct Order {
    1: string id
    2: string user_id
    3: double amount
    4: i32 status
}

struct CreateOrderReq {
    1: string id
    2: string user_id
    3: double amount
}

struct OrderResp {
    1: i32 code
    2: string message
    3: Order data
}

struct PayOrderReq {
    1: string id
}

struct ShipOrderReq {
    1: string id
}

// User
struct User {
    1: string id
    2: string name
    3: string email
    4: i32 level
}

struct RegisterUserReq {
    1: string id
    2: string name
    3: string email
}

struct UserResp {
    1: i32 code
    2: string message
    3: User data
}

struct UpgradeUserReq {
    1: string id
}

service GS_PROJECT_NAMEService {
    PingResp Ping(1: PingReq req)
    OrderResp CreateOrder(1: CreateOrderReq req)
    OrderResp PayOrder(1: PayOrderReq req)
    OrderResp ShipOrder(1: ShipOrderReq req)
    UserResp RegisterUser(1: RegisterUserReq req)
    UserResp UpgradeUser(1: UpgradeUserReq req)
}
