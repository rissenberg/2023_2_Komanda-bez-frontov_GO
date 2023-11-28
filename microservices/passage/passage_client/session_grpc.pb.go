// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package passage

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// FormPassageClient is the client API for FormPassage service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type FormPassageClient interface {
	Pass(ctx context.Context, in *Passage, opts ...grpc.CallOption) (*Nothing, error)
}

type formPassageClient struct {
	cc grpc.ClientConnInterface
}

func NewFormPassageClient(cc grpc.ClientConnInterface) FormPassageClient {
	return &formPassageClient{cc}
}

func (c *formPassageClient) Pass(ctx context.Context, in *Passage, opts ...grpc.CallOption) (*Nothing, error) {
	out := new(Nothing)
	err := c.cc.Invoke(ctx, "/passage.FormPassage/Pass", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FormPassageServer is the server API for FormPassage service.
// All implementations must embed UnimplementedFormPassageServer
// for forward compatibility
type FormPassageServer interface {
	Pass(context.Context, *Passage) (*Nothing, error)
	mustEmbedUnimplementedFormPassageServer()
}

// UnimplementedFormPassageServer must be embedded to have forward compatible implementations.
type UnimplementedFormPassageServer struct {
}

func (UnimplementedFormPassageServer) Pass(context.Context, *Passage) (*Nothing, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Pass not implemented")
}
func (UnimplementedFormPassageServer) mustEmbedUnimplementedFormPassageServer() {}

// UnsafeFormPassageServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to FormPassageServer will
// result in compilation errors.
type UnsafeFormPassageServer interface {
	mustEmbedUnimplementedFormPassageServer()
}

func RegisterFormPassageServer(s grpc.ServiceRegistrar, srv FormPassageServer) {
	s.RegisterService(&FormPassage_ServiceDesc, srv)
}

func _FormPassage_Pass_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Passage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FormPassageServer).Pass(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/passage.FormPassage/Pass",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FormPassageServer).Pass(ctx, req.(*Passage))
	}
	return interceptor(ctx, in, info, handler)
}

// FormPassage_ServiceDesc is the grpc.ServiceDesc for FormPassage service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var FormPassage_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "passage.FormPassage",
	HandlerType: (*FormPassageServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Pass",
			Handler:    _FormPassage_Pass_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "session.proto",
}
