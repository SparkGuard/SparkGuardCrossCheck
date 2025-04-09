package middleware

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func NewAuthInterceptor(token string) func(context.Context, string, any, any,
	*grpc.ClientConn, grpc.UnaryInvoker, ...grpc.CallOption) error {

	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		// Добавление токена авторизации.
		md := metadata.New(map[string]string{"authorization": token})
		newContext := metadata.NewOutgoingContext(ctx, md)

		return invoker(newContext, method, req, reply, cc, opts...)
	}
}
