// Package raftgrpc implements gRPC transportation layer for raft.
package raftgrpc

import (
	"context"

	itransport "github.com/shaj13/raft/internal/transport"
	"github.com/shaj13/raft/internal/transport/raftgrpc"
	"github.com/shaj13/raft/internal/transport/raftgrpc/pb"
	"github.com/shaj13/raft/raftlog"
	"github.com/shaj13/raft/transport"
	"google.golang.org/grpc"
)

func init() {
	Register()
}

type config struct {
	copts func(context.Context) []grpc.CallOption
	dopts func(context.Context) []grpc.DialOption
}

// Option configures grpc using the functional options paradigm popularized by Rob Pike and Dave Cheney.
// If you're unfamiliar with this style,
// see https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html and
// https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis.
type Option interface {
	apply(c *config)
}

// OptionFunc implements Option interface.
type optionFunc func(c *config)

// Apply the configuration to the provided strategy.
func (fn optionFunc) apply(c *config) {
	fn(c)
}

// WithCallOptions configures grpc client call from the given options.
func WithCallOptions(opts ...grpc.CallOption) Option {
	return optionFunc(func(c *config) {
		c.copts = func(c context.Context) []grpc.CallOption {
			return opts
		}
	})
}

// WithDialOptions configures grpc dial from the given options.
func WithDialOptions(opts ...grpc.DialOption) Option {
	return optionFunc(func(c *config) {
		c.dopts = func(c context.Context) []grpc.DialOption {
			return opts
		}
	})
}

// Register registers the gRPC for use with all clients and servers communication.
//
// NOTE: this function must only be called during initialization time (i.e. in
// an init() function), and is not thread-safe.
func Register(opts ...Option) {
	c := new(config)
	c.copts = func(c context.Context) []grpc.CallOption { return nil }
	c.dopts = func(c context.Context) []grpc.DialOption { return nil }

	for _, opt := range opts {
		opt.apply(c)
	}

	dialer := raftgrpc.Dialer(c.dopts, c.copts)
	nh := raftgrpc.NewHandler

	itransport.GRPC.Register(nh, dialer)
}

// RegisterHandler registers transport handler and its implementation to the gRPC server.
func RegisterHandler(s *grpc.Server, h transport.Handler) {
	if rs, ok := h.(pb.RaftServer); ok {
		pb.RegisterRaftServer(s, rs)
		return
	}

	raftlog.Fatalf("raft.grpc: type %T does not implement gRPC transport handler", h)
}
