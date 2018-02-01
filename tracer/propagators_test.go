package tracer

import (
	"net/http"
	"strconv"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
)

func TestOpenTracerPropagationDefaults(t *testing.T) {
	assert := assert.New(t)

	tracer := New()
	root := tracer.StartSpan("web.request")
	ctx := root.Context()
	headers := http.Header{}

	// inject the SpanContext
	carrier := opentracing.HTTPHeadersCarrier(headers)
	err := tracer.Inject(ctx, opentracing.HTTPHeaders, carrier)
	assert.Nil(err)

	// retrieve the SpanContext
	propagated, err := tracer.Extract(opentracing.HTTPHeaders, carrier)
	assert.Nil(err)

	tCtx, ok := ctx.(SpanContext)
	assert.True(ok)
	tPropagated, ok := propagated.(SpanContext)
	assert.True(ok)

	// compare if there is a Context match
	assert.Equal(tCtx.traceID, tPropagated.traceID)
	assert.Equal(tCtx.spanID, tPropagated.spanID)

	// ensure a child can be created
	child := tracer.StartSpan("db.query", opentracing.ChildOf(propagated))
	tRoot, ok := root.(*OpenSpan)
	assert.True(ok)
	tChild, ok := child.(*OpenSpan)
	assert.True(ok)

	assert.NotEqual(uint64(0), tChild.Span.TraceID)
	assert.NotEqual(uint64(0), tChild.Span.SpanID)
	assert.Equal(tRoot.Span.SpanID, tChild.Span.ParentID)
	assert.Equal(tRoot.Span.TraceID, tChild.Span.ParentID)

	tid := strconv.FormatUint(tRoot.Span.TraceID, 10)
	pid := strconv.FormatUint(tRoot.Span.SpanID, 10)

	// hardcode header names to fail test if defaults are changed
	assert.Equal(headers.Get("x-datadog-trace-id"), tid)
	assert.Equal(headers.Get("x-datadog-parent-id"), pid)
}

func TestOpenTracerTextMapPropagationHeader(t *testing.T) {
	assert := assert.New(t)

	textMapPropagator := NewTextMapPropagator("bg-", "tid", "pid")
	tracer := New(WithTextMapPropagator(textMapPropagator))

	root := tracer.StartSpan("web.request").SetBaggageItem("item", "x").(*OpenSpan)
	ctx := root.Context()
	headers := http.Header{}

	carrier := opentracing.HTTPHeadersCarrier(headers)
	err := tracer.Inject(ctx, opentracing.HTTPHeaders, carrier)
	assert.Nil(err)

	tid := strconv.FormatUint(root.Span.TraceID, 10)
	pid := strconv.FormatUint(root.Span.SpanID, 10)

	assert.Equal(headers.Get("tid"), tid)
	assert.Equal(headers.Get("pid"), pid)
	assert.Equal(headers.Get("bg-item"), "x")
}
