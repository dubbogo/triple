# Triple-go
dubbo protocol v3 (alias: triple) in Go

[中文版README](./README_zh.md)

### Summary

Triple-go is a golang network package that based on http2, used by [Dubbo-go](https://github.com/apache/dubbo-go) 3.0.

It is an amazing (grpc+) protocol, compatible with Dubbo 3.0 and grpc-go. has great interoperability with grpc-go and [Dubbo](https://github.com/apache/dubbo) 3.0.

### Feature

- Use standard proto IDL to define your dubbo-go service and realize cross-language intercommunication with dubbo-java.
- Perfect interoperability with your existing grpc services, allowing you to experience a framework with both dubbo service management capabilities and grpc compatibility.
- Provide two-way streaming RPC calls and communicate with grpc.

### Architecture
![triple-go-arch](https://dubbogo.github.io/img/doc/triple-go-arch.jpg)

### Triple Protocol

- Alibaba Cloud official introduction article: [《Dubbo 3.0-Open the next generation of cloud-native microservices》](https://developer.aliyun.com/article/770964?utm_content=g_1000175535)

- Http2 protocol header extension fields:
  - Requeset header

    tri-service-version: dubbo app version

    tri-service-group: dubbo app group

    tri-req-id: request id

    tri-trace-traceid: trace id

    tri-trace-rpcid: span id

    tri-trace-proto-bin: trace context binary data

    tri-unit-info: cluster information

  - Response header（trailer fields）

    grpc-status: grpc status code

    grpc-message: error message

    trace-proto-bin: trace binary data

### Docs
[Triple-go docs](./docs/README.md)

