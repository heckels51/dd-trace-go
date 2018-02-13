package opentracer

import (
	"testing"

	"github.com/DataDog/dd-trace-go/dd"
	"github.com/DataDog/dd-trace-go/tracer/internal"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
)

func TestStart(t *testing.T) {
	assert := assert.New(t)
	Start()
	dd, ok := internal.GlobalTracer.(dd.Tracer)
	assert.True(ok)
	ot, ok := opentracing.GlobalTracer().(*opentracer)
	assert.True(ok)
	assert.Equal(ot.Tracer, dd)
}
