# Triple-go
Dubbo 3.0 协议 (别名: Triple) 的go语言版本。

### 简介

Triple-go 是一个基于http2协议的go语言协议库，在 [Dubbo-go](https://github.com/apache/dubbo-go)  3.0 中作为全新的feature被使用。

这是一个 grpc 扩展协议库，它与grpc协议互通，并且和 [Dubbo-java](https://github.com/apache/dubbo) 的3.0协议完全兼容。

### 功能

- 使用标准 proto IDL 定义您的 dubbo-go 服务，并实现与 dubbo-java 跨语言互通。
- 与您现有的 grpc 服务完美互通，使您体验兼具dubbo服务治理能力 和 grpc兼容性的框架。
- 提供双向流式 RPC 调用，并与grpc互通。

### Triple协议

- 阿里云官方介绍文章：[《Dubbo 3.0 - 开启下一代云原生微服务》](https://developer.aliyun.com/article/770964?utm_content=g_1000175535)

- Http2协议头扩展字段：

  - 请求头

    tri-service-version: dubbo 应用版本号 

    tri-service-group: dubbo 应用 group

    tri-req-id: 请求id

    tri-trace-traceid: trace id

    tri-trace-rpcid: span id

    tri-trace-proto-bin: trace上下文二进制信息

    tri-unit-info: 集群信息

  - 返回头（trailer 返回头字段）

    grpc-status: grpc 状态码，与 grpc 兼容

    grpc-message: 报错信息

    trace-proto-bin: trace 二进制信息

### 文档

[Triple-go 文档](./docs/README_zh.md)

