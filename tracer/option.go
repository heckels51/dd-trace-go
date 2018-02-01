package tracer

import (
	"os"
	"path/filepath"
)

type config struct {
	// debug, when true, writes details to logs.
	debug bool

	// serviceName specifies the name of this application.
	serviceName string

	// sampleRate sets the Tracer sample rate (ext/priority.go).
	sampleRate float64

	// agentAddr specifies the hostname and  of the agent where the traces
	// are sent to.
	agentAddr string

	// globalTags holds a set of tags that will be automatically applied to
	// all spans.
	globalTags map[string]interface{}

	// textMapPropagator is the TextMap propagator used for Context propagation.
	textMapPropagator Propagator

	// binaryPropagator is the Binary propagator used for Context propagation.
	binaryPropagator Propagator
}

type Option func(*config)

func defaults(c *config) {
	c.serviceName = filepath.Base(os.Args[0])
	c.sampleRate = 1
	c.agentAddr = "localhost:8126"
	c.textMapPropagator = NewTextMapPropagator("", "", "")
}

func WithDebugMode(enabled bool) Option {
	return func(c *config) {
		c.debug = enabled
	}
}

func WithServiceName(name string) Option {
	return func(c *config) {
		c.serviceName = name
	}
}

func WithSampleRate(rate float64) Option {
	return func(c *config) {
		c.sampleRate = rate
	}
}

func WithAgentAddr(addr string) Option {
	return func(c *config) {
		c.agentAddr = addr
	}
}

func WithGlobalTag(k string, v interface{}) Option {
	return func(c *config) {
		if c.globalTags == nil {
			c.globalTags = make(map[string]interface{})
		}
		c.globalTags[k] = v
	}
}

// DOC: must be pairs with keys as string otherwise panic.
func WithGlobalTags(kv ...interface{}) Option {
	if n := len(kv); n < 2 || n%2 != 0 {
		panic("uneven number of arguments supplied")
	}
	return func(c *config) {
		for i := 0; i <= len(kv)-2; i = i + 2 {
			k, ok := kv[i].(string)
			if !ok {
				panic("all keys must be strings")
			}
			WithGlobalTag(k, kv[i+1])(c)
		}
	}
}

func WithTextMapPropagator(p Propagator) Option {
	return func(c *config) {
		c.textMapPropagator = p
	}
}

func WithBinaryPropagator(p Propagator) Option {
	return func(c *config) {
		c.binaryPropagator = p
	}
}
