# netutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. Currently the local-IPv4
lookup used by the framework's registration and startup logs.

## 1. Responsibilities & Boundaries

- Give the framework one call to get a stable "how do I appear on the
  network?" string. Used for service registration, log tagging, and
  actuator info.
- Not a networking framework. No interface enumeration, no CIDR matching,
  no address parsing beyond what `net` already offers.

## 2. Design Notes

- `sync.Once`-backed cache: the answer is stable across the process lifetime,
  which matches the framework's assumption that host addressing is fixed
  after boot.
- Errors are hidden behind the `"0.0.0.0"` sentinel to keep the API
  string-typed. The trade-off: silent misconfiguration. Callers that need
  hard failures should probe `net.InterfaceAddrs` directly.
- IPv4-only. IPv6 is not supported here because most Go-Spring consumers
  (Nacos / etcd / Consul style discovery, log lines) still key on IPv4.
