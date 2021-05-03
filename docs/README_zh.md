# Triple Docs 中文版

### 1. 项目结构

- triple/docs

  文档

- triple/internal/

  triple 内部模块

  - codec/

    打解包模块：

    codec.go: 序列化组件

    header.go: Triple协议字段定义组件

    package.go: 数据帧拆包/打包模块

  - codes/

    grpc 状态码

  - message/

    内部数据传输结构/类型定义

  - status/

    grpc status

  - stream/

    双向数据流结构定义

    processor.go: 执行RPC过程

    stream.go: 框架侧双向数据流定义

    user_stream.go: 用户侧双向数据流定义

  - syscall: 

    压测依赖模块

- triple/pkg/

  triple 公共模块

  - common/

    公共结构/interface/常量定义

  - config/

    triple 配置

  - triple/

    triple 接口模块

    dubbo3_client.go: triple 网络客户端

    dubbo3_conn.go: triple conn 结构，用于grpc stub接口兼容

    dubbo3_server.go: triple 网络服务端

    http2.go: Http2 请求/服务 模块

### 2. 接口说明

- **服务端接口**

  **新建Server**

  ```go
  // NewTripleServer can create Server with some user impl providers stored in @serviceMap
  // @serviceMap should be sync.Map: "interfaceKey" -> Dubbo3GrpcService
  func NewTripleServer(opt *config.Option, serviceMap *sync.Map, opt *config.Option) 
  ```

  参数作用：

  ​	serviceMap 的key为服务id，即dubbo interfaceKey。value为暴露的grpc扩展服务，需实现接口Dubbo3GrpcService，可通过proto-gen工具生成stub后，由用户实现，并放入serviceMap。serviceMap中可放入多个service, 即可实现单app多接口启动。

  ​	opt为服务暴露配置，可传入nil使用默认配置启动。

  server端启动例子可见dubbo-go/protocol/dubbo3/dubbo3_protocol.go: Export()

  **开启Server**

  ```go
  func (t *TripleServer) Start() 
  ```

  **关闭Server**

  ```go
  func (t *TripleServer) Stop()
  ```

  

- **客户端接口**

  **新建Client**

  ```go
  // NewTripleClient create triple client
  // it's return tripleClient , contains invoker, and contain triple conn
  // @impl must have method: GetDubboStub(cc *dubbo3.TripleConn) interface{}, to be capable with grpc
  // @opt is used init http2 controller, if it's nil, use the default config
  func NewTripleClient(url *config.Option, impl interface{}, opt *config.Option) (*TripleClient, error) {
  ```

  参数作用：

  ​	opt为服务暴露配置，可传入nil使用默认配置启动。

  ​	impl为实现 GetDubboStub 方法的客户端结构，该方法由客户端用户自行实现，需返回自动生成stub的XXXDubbo3Client 结构，用于客户端打解包通信。

  例子：

  ```go
  type DubboGreeterImpl struct {
  	Dubbo3SayHello2 func(ctx context.Context, in *dubbo32.Dubbo3HelloRequest, out *dubbo32.Dubbo3HelloReply) error
  	BigUnaryTest func(ctx context.Context, in*dubbo32.BigData) (*dubbo32.BigData,error)
  }
  
  func (u *GrpcGreeterImpl) Reference() string {
  	return "GrpcGreeterImpl"
  }
  
  // GetDubboStub 方法返回stub中生成的 Dubbo Client
  func (u *DubboGreeterImpl) GetDubboStub(cc *dubbo3.TripleConn) dubbo32.Dubbo3GreeterClient {
  	return dubbo32.NewDubbo3GreeterDubbo3Client(cc)
  }
  ```

  **普通RPC调用**

  ```go
  // Request call h2Controller to send unary rpc req to server
  // @path is /interfaceKey/functionName e.g. /com.apache.dubbo.sample.basic.IGreeter/BigUnaryTest
  // @arg is request body
  func (t *TripleClient) Request(ctx context.Context, path string, arg, reply proto.Message) error 
  ```

  参数作用：

  ​	ctx为当前请求上下文，可考虑与triple协议字段相关联

  ​	path为 http2 path 参数： /interfaceKey/functionName 结构，服务端会根据path定位当前app提供的具体服务对应函数

  ​	reply为返回值.

  

  **流式RPC调用**

  ```go
  // StreamRequest call h2Controller to send streaming request to sever, to start link.
  // @path is /interfaceKey/functionName e.g. /com.apache.dubbo.sample.basic.IGreeter/BigStreamTest
  func (t *TripleClient) StreamRequest(ctx context.Context, path string) (grpc.ClientStream, error)
  ```

  参数作用：

  ​	ctx为当前请求上下文，可考虑与triple协议字段相关联

  ​	path为 http2 path 参数： /interfaceKey/functionName 结构，服务端会根据path定位当前app提供的具体服务对应函数

  函数返回grpc的client stream结构，用于与用户交互

- GRPC stub 接口

  在dubbo-go的实现中，需要将上述 client 暴露的接口以 TripleConn 的形式注册到grpc stub上，可看到TripleConn结构提供了Invoke （普通调用）和NewStream（流式调用）方法，用于传入grpc stub。

  ```go
  Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error
  
  NewStream(ctx context.Context, method string, opts ...grpc.CallOption) (grpc.ClientStream, error)
  ```

  