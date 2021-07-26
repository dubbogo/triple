
package pkg

import (
	context "context"
	fmt "fmt"
	math "math"
)

import (
	"dubbo.apache.org/dubbo-go/v3/protocol"
	dgrpc "dubbo.apache.org/dubbo-go/v3/protocol/dubbo3"
	"dubbo.apache.org/dubbo-go/v3/protocol/invocation"

	proto "github.com/golang/protobuf/proto"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

import (
	tripleConstant "github.com/dubbogo/triple/pkg/common/constant"
	dubbo3 "github.com/dubbogo/triple/pkg/triple"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// The request message containing the user's name.
type HelloRequest struct {
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *HelloRequest) Reset()         { *m = HelloRequest{} }
func (m *HelloRequest) String() string { return proto.CompactTextString(m) }
func (*HelloRequest) ProtoMessage()    {}
func (*HelloRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_17b8c58d586b62f2, []int{0}
}

func (m *HelloRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_HelloRequest.Unmarshal(m, b)
}
func (m *HelloRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_HelloRequest.Marshal(b, m, deterministic)
}
func (m *HelloRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_HelloRequest.Merge(m, src)
}
func (m *HelloRequest) XXX_Size() int {
	return xxx_messageInfo_HelloRequest.Size(m)
}
func (m *HelloRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_HelloRequest.DiscardUnknown(m)
}

var xxx_messageInfo_HelloRequest proto.InternalMessageInfo

func (m *HelloRequest) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

// The response message containing the greetings
type User struct {
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Id                   string   `protobuf:"bytes,2,opt,name=id,proto3" json:"id,omitempty"`
	Age                  int32    `protobuf:"varint,3,opt,name=age,proto3" json:"age,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *User) Reset()         { *m = User{} }
func (m *User) String() string { return proto.CompactTextString(m) }
func (*User) ProtoMessage()    {}
func (*User) Descriptor() ([]byte, []int) {
	return fileDescriptor_17b8c58d586b62f2, []int{1}
}

func (m *User) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_User.Unmarshal(m, b)
}
func (m *User) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_User.Marshal(b, m, deterministic)
}
func (m *User) XXX_Merge(src proto.Message) {
	xxx_messageInfo_User.Merge(m, src)
}
func (m *User) XXX_Size() int {
	return xxx_messageInfo_User.Size(m)
}
func (m *User) XXX_DiscardUnknown() {
	xxx_messageInfo_User.DiscardUnknown(m)
}

var xxx_messageInfo_User proto.InternalMessageInfo

func (m *User) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *User) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *User) GetAge() int32 {
	if m != nil {
		return m.Age
	}
	return 0
}

func init() {
	proto.RegisterType((*HelloRequest)(nil), "main.HelloRequest")
	proto.RegisterType((*User)(nil), "main.User")
}

func init() { proto.RegisterFile("helloworld.proto", fileDescriptor_17b8c58d586b62f2) }

var fileDescriptor_17b8c58d586b62f2 = []byte{
	// 227 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0xc8, 0x48, 0xcd, 0xc9,
	0xc9, 0x2f, 0xcf, 0x2f, 0xca, 0x49, 0xd1, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0xc9, 0x4d,
	0xcc, 0xcc, 0x53, 0x52, 0xe2, 0xe2, 0xf1, 0x00, 0xc9, 0x04, 0xa5, 0x16, 0x96, 0xa6, 0x16, 0x97,
	0x08, 0x09, 0x71, 0xb1, 0xe4, 0x25, 0xe6, 0xa6, 0x4a, 0x30, 0x2a, 0x30, 0x6a, 0x70, 0x06, 0x81,
	0xd9, 0x4a, 0x36, 0x5c, 0x2c, 0xa1, 0xc5, 0xa9, 0x45, 0xd8, 0xe4, 0x84, 0xf8, 0xb8, 0x98, 0x32,
	0x53, 0x24, 0x98, 0xc0, 0x22, 0x4c, 0x99, 0x29, 0x42, 0x02, 0x5c, 0xcc, 0x89, 0xe9, 0xa9, 0x12,
	0xcc, 0x0a, 0x8c, 0x1a, 0xac, 0x41, 0x20, 0xa6, 0x51, 0x3e, 0x17, 0xbb, 0x7b, 0x51, 0x6a, 0x6a,
	0x49, 0x6a, 0x91, 0x90, 0x19, 0x17, 0x5f, 0x70, 0x62, 0x25, 0xd8, 0xbe, 0xe0, 0x92, 0xa2, 0xd4,
	0xc4, 0x5c, 0x21, 0x21, 0x3d, 0x90, 0x2b, 0xf4, 0x90, 0x9d, 0x20, 0xc5, 0x05, 0x11, 0x03, 0x59,
	0xa9, 0xc4, 0xa0, 0xc1, 0x68, 0xc0, 0x28, 0xa4, 0xc3, 0xc5, 0x01, 0xd3, 0x47, 0x58, 0x87, 0x93,
	0x1d, 0x97, 0x74, 0x66, 0xbe, 0x5e, 0x7a, 0x51, 0x41, 0xb2, 0x5e, 0x6a, 0x45, 0x62, 0x6e, 0x41,
	0x4e, 0x6a, 0xb1, 0x1e, 0xc2, 0xf7, 0x4e, 0xfc, 0x60, 0xad, 0xe1, 0x20, 0x76, 0x00, 0x28, 0x20,
	0x02, 0x18, 0xa3, 0xd8, 0xf4, 0xf4, 0xad, 0x0b, 0xb2, 0xd3, 0x17, 0x31, 0x31, 0x7b, 0xf8, 0x84,
	0x27, 0xb1, 0x81, 0xc3, 0xc7, 0x18, 0x10, 0x00, 0x00, 0xff, 0xff, 0x57, 0x88, 0x7e, 0x7f, 0x33,
	0x01, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// GreeterClient is the client API for Greeter service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type GreeterClient interface {
	// Sends a greeting
	SayHelloStream(ctx context.Context, opts ...grpc.CallOption) (Greeter_SayHelloStreamClient, error)
	SayHello(ctx context.Context, in *HelloRequest, opts ...grpc.CallOption) (*User, error)
}

type greeterClient struct {
	cc grpc.ClientConnInterface
}

func NewGreeterClient(cc grpc.ClientConnInterface) GreeterClient {
	return &greeterClient{cc}
}

func (c *greeterClient) SayHelloStream(ctx context.Context, opts ...grpc.CallOption) (Greeter_SayHelloStreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Greeter_serviceDesc.Streams[0], "/main.Greeter/SayHelloStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &greeterSayHelloStreamClient{stream}
	return x, nil
}

type Greeter_SayHelloStreamClient interface {
	Send(*HelloRequest) error
	Recv() (*User, error)
	grpc.ClientStream
}

type greeterSayHelloStreamClient struct {
	grpc.ClientStream
}

func (x *greeterSayHelloStreamClient) Send(m *HelloRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *greeterSayHelloStreamClient) Recv() (*User, error) {
	m := new(User)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *greeterClient) SayHello(ctx context.Context, in *HelloRequest, opts ...grpc.CallOption) (*User, error) {
	out := new(User)
	err := c.cc.Invoke(ctx, "/main.Greeter/SayHello", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GreeterServer is the server API for Greeter service.
type GreeterServer interface {
	// Sends a greeting
	SayHelloStream(Greeter_SayHelloStreamServer) error
	SayHello(context.Context, *HelloRequest) (*User, error)
}

// UnimplementedGreeterServer can be embedded to have forward compatible implementations.
type UnimplementedGreeterServer struct {
}

func (*UnimplementedGreeterServer) SayHelloStream(srv Greeter_SayHelloStreamServer) error {
	return status.Errorf(codes.Unimplemented, "method SayHelloStream not implemented")
}
func (*UnimplementedGreeterServer) SayHello(ctx context.Context, req *HelloRequest) (*User, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SayHello not implemented")
}

func RegisterGreeterServer(s *grpc.Server, srv GreeterServer) {
	s.RegisterService(&_Greeter_serviceDesc, srv)
}

func _Greeter_SayHelloStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(GreeterServer).SayHelloStream(&greeterSayHelloStreamServer{stream})
}

type Greeter_SayHelloStreamServer interface {
	Send(*User) error
	Recv() (*HelloRequest, error)
	grpc.ServerStream
}

type greeterSayHelloStreamServer struct {
	grpc.ServerStream
}

func (x *greeterSayHelloStreamServer) Send(m *User) error {
	return x.ServerStream.SendMsg(m)
}

func (x *greeterSayHelloStreamServer) Recv() (*HelloRequest, error) {
	m := new(HelloRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Greeter_SayHello_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HelloRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(GreeterServer).SayHello(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/main.Greeter/SayHello",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(GreeterServer).SayHello(ctx, req.(*HelloRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Greeter_serviceDesc = grpc.ServiceDesc{
	ServiceName: "main.Greeter",
	HandlerType: (*GreeterServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SayHello",
			Handler:    _Greeter_SayHello_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SayHelloStream",
			Handler:       _Greeter_SayHelloStream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "helloworld.proto",
}

type greeterDubbo3Client struct {
	cc *dubbo3.TripleConn
}

func NewGreeterDubbo3Client(cc *dubbo3.TripleConn) GreeterClient {
	return &greeterDubbo3Client{cc}
}
func (c *greeterDubbo3Client) SayHelloStream(ctx context.Context, opt ...grpc.CallOption) (Greeter_SayHelloStreamClient, error) {
	interfaceKey := ctx.Value(tripleConstant.InterfaceKey).(string)
	stream, err := c.cc.NewStream(ctx, "/"+interfaceKey+"/SayHelloStream", opt...)
	if err != nil {
		return nil, err
	}
	x := &greeterSayHelloStreamClient{stream}
	return x, nil
}
func (c *greeterDubbo3Client) SayHello(ctx context.Context, in *HelloRequest, opt ...grpc.CallOption) (*User, error) {
	out := new(User)
	interfaceKey := ctx.Value(tripleConstant.InterfaceKey).(string)
	err := c.cc.Invoke(ctx, "/"+interfaceKey+"/SayHello", in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GreeterClientImpl is the client API for Greeter service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type GreeterClientImpl struct {
	// Sends a greeting
	SayHelloStream func(ctx context.Context) (Greeter_SayHelloStreamClient, error)
	SayHello       func(ctx context.Context, in *HelloRequest, out *User) error
}

func (c *GreeterClientImpl) Reference() string {
	return "greeterImpl"
}

func (c *GreeterClientImpl) GetDubboStub(cc *dubbo3.TripleConn) GreeterClient {
	return NewGreeterDubbo3Client(cc)
}

type GreeterProviderBase struct {
	proxyImpl protocol.Invoker
}

func (s *GreeterProviderBase) SetProxyImpl(impl protocol.Invoker) {
	s.proxyImpl = impl
}

func (s *GreeterProviderBase) GetProxyImpl() protocol.Invoker {
	return s.proxyImpl
}

func (c *GreeterProviderBase) Reference() string {
	return "greeterImpl"
}

func _DUBBO_Greeter_SayHelloStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	_, ok := srv.(dgrpc.Dubbo3GrpcService)
	invo := invocation.NewRPCInvocation("SayHelloStream", nil, nil)
	if !ok {
		fmt.Println(invo)
	}
	return srv.(GreeterServer).SayHelloStream(&greeterSayHelloStreamServer{stream})
}

func _DUBBO_Greeter_SayHello_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HelloRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	base := srv.(dgrpc.Dubbo3GrpcService)
	args := []interface{}{}
	args = append(args, in)
	invo := invocation.NewRPCInvocation("SayHello", args, nil)
	if interceptor == nil {
		result := base.GetProxyImpl().Invoke(ctx, invo)
		return result.Result(), result.Error()
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/main.Greeter/SayHello",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		result := base.GetProxyImpl().Invoke(context.Background(), invo)
		return result.Result(), result.Error()
	}
	return interceptor(ctx, in, info, handler)
}

func (s *GreeterProviderBase) ServiceDesc() *grpc.ServiceDesc {
	return &grpc.ServiceDesc{
		ServiceName: "main.Greeter",
		HandlerType: (*GreeterServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "SayHello",
				Handler:    _DUBBO_Greeter_SayHello_Handler,
			},
		},
		Streams: []grpc.StreamDesc{
			{
				StreamName:    "SayHelloStream",
				Handler:       _DUBBO_Greeter_SayHelloStream_Handler,
				ServerStreams: true,
				ClientStreams: true,
			},
		},
		Metadata: "helloworld.proto",
	}
}
