package http

import (
	"context"
	"net/http"

	"github.com/shaj13/raftkit/internal/log"
	itransport "github.com/shaj13/raftkit/internal/transport"
	rafthttp "github.com/shaj13/raftkit/internal/transport/http"
	"github.com/shaj13/raftkit/transport"
)

func init() {
	Register()
}

type config struct {
	tr       func(context.Context) http.RoundTripper
	basePath string
}

// Option configures http using the functional options paradigm popularized by Rob Pike and Dave Cheney.
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

// WithTransport optionally specifies an http.RoundTripper for the client
// to use when it makes a request.
// Default: http.DefaultTransport.
func WithTransport(tr http.RoundTripper) Option {
	return optionFunc(func(c *config) {
		c.tr = func(c context.Context) http.RoundTripper {
			return tr
		}
	})
}

// WithBasePath specifies the HTTP path that will serve raft requests.
// Default: "/_raft/".
func WithBasePath(tr http.RoundTripper) Option {
	return optionFunc(func(c *config) {
		c.tr = func(c context.Context) http.RoundTripper {
			return tr
		}
	})
}

// Register registers the http for use with all clients and servers communication.
//
// NOTE: this function must only be called during initialization time (i.e. in
// an init() function), and is not thread-safe.
func Register(opts ...Option) {
	c := new(config)
	c.tr = func(c context.Context) http.RoundTripper { return http.DefaultTransport }
	c.basePath = "/_raft/"

	for _, opt := range opts {
		opt.apply(c)
	}

	dialer := rafthttp.Dialer(c.tr, c.basePath)
	nh := rafthttp.NewHandlerFunc(c.basePath)

	itransport.HTTP.Register(nh, dialer)
}

// Handler return's http.Handler for http transport server.
func Handler(h transport.Handler) http.Handler {
	if h, ok := h.(http.Handler); ok {
		return h
	}

	log.Fatalf("raft.http: type %T does not implement transport handler", h)
	return nil
}
