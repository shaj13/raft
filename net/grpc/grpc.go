package grpc

import (
	"github.com/shaj13/raftkit/internal/net"
	"google.golang.org/grpc"
)

func init() {
	Register()
}

type config struct {
	callOpts []grpc.CallOption
	dialOpts []grpc.DialOption
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
		c.callOpts = opts
	})
}

// WithDialOptions configures grpc dial from the given options.
func WithDialOptions(opts ...grpc.DialOption) Option {
	return optionFunc(func(c *config) {
		c.dialOpts = opts
	})
}

// Register registers the gRPC for use with all clients and servers communication.
//
// NOTE: this function must only be called during initialization time (i.e. in
// an init() function), and is not thread-safe.
func Register(opts ...Option) {
	c := new(config)

	for _, opt := range opts {
		opt.apply(c)
	}

	// need to create server annd dial.
	net.GRPC.Register(nil, nil)
}
