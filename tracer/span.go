package tracer

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

var _ opentracing.Span = (*Span)(nil)

// Tracer provides access to the `Tracer`` that created this Span.
func (s *Span) Tracer() opentracing.Tracer { return s.tracer }

// Context yields the SpanContext for this Span. Note that the return
// value of Context() is still valid after a call to Span.Finish(), as is
// a call to Span.Context() after a call to Span.Finish().
func (s *Span) Context() opentracing.SpanContext {
	s.RLock()
	defer s.RUnlock()

	return s.context
}

// SetBaggageItem sets a key:value pair on this Span and its SpanContext
// that also propagates to descendants of this Span.
func (s *Span) SetBaggageItem(key, val string) opentracing.Span {
	s.Lock()
	defer s.Unlock()

	s.context = s.context.WithBaggageItem(key, val)
	return s
}

// BaggageItem gets the value for a baggage item given its key. Returns the empty string
// if the value isn't found in this Span.
func (s *Span) BaggageItem(key string) string {
	s.Lock()
	defer s.Unlock()

	return s.context.baggage[key]
}

// SetTag adds a tag to the span, overwriting pre-existing values for
// the given `key`.
func (s *Span) SetTag(key string, value interface{}) opentracing.Span {
	switch key {
	case ServiceName:
		s.Lock()
		defer s.Unlock()
		s.Service = fmt.Sprint(value)
	case ResourceName:
		s.Lock()
		defer s.Unlock()
		s.Resource = fmt.Sprint(value)
	case SpanType:
		s.Lock()
		defer s.Unlock()
		s.Type = fmt.Sprint(value)
	case Error:
		switch v := value.(type) {
		case nil:
			// no error
		case error:
			s.SetError(v)
		default:
			s.SetError(fmt.Errorf("%v", v))
		}
	default:
		// NOTE: locking is not required because the `SetMeta` is
		// already thread-safe
		s.SetMeta(key, fmt.Sprint(value))
	}
	return s
}

// FinishWithOptions is like Finish() but with explicit control over
// timestamps and log data.
func (s *Span) FinishWithOptions(options opentracing.FinishOptions) {
	if options.FinishTime.IsZero() {
		options.FinishTime = time.Now().UTC()
	}

	s.FinishWithTime(options.FinishTime.UnixNano())
}

// SetOperationName sets or changes the operation name.
func (s *Span) SetOperationName(operationName string) opentracing.Span {
	s.Lock()
	defer s.Unlock()

	s.Name = operationName
	return s
}

// LogFields is an efficient and type-checked way to record key:value
// logging data about a Span, though the programming interface is a little
// more verbose than LogKV().
func (s *Span) LogFields(fields ...log.Field) {
	// TODO: implementation missing
}

// LogKV is a concise, readable way to record key:value logging data about
// a span, though unfortunately this also makes it less efficient and less
// type-safe than LogFields().
func (s *Span) LogKV(keyVals ...interface{}) {
	// TODO: implementation missing
}

// LogEvent is deprecated: use LogFields or LogKV
func (s *Span) LogEvent(event string) {
	// TODO: implementation missing
}

// LogEventWithPayload deprecated: use LogFields or LogKV
func (s *Span) LogEventWithPayload(event string, payload interface{}) {
	// TODO: implementation missing
}

// Log is deprecated: use LogFields or LogKV
func (s *Span) Log(data opentracing.LogData) {
	// TODO: implementation missing
}

// OLD ////////////////////////////////

const (
	errorMsgKey   = "error.msg"
	errorTypeKey  = "error.type"
	errorStackKey = "error.stack"

	samplingPriorityKey = "_sampling_priority_v1"
)

// Span represents a computation. Callers must call Finish when a span is
// complete to ensure it's submitted.
//
//	span := tracer.NewRootSpan("web.request", "datadog.com", "/user/{id}")
//	defer span.Finish()  // or FinishWithErr(err)
//
// In general, spans should be created with the tracer.NewSpan* functions,
// so they will be submitted on completion.
type Span struct {
	// Name is the name of the operation being measured. Some examples
	// might be "http.handler", "fileserver.upload" or "video.decompress".
	// Name should be set on every span.
	Name string `json:"name"`

	// Service is the name of the process doing a particular job. Some
	// examples might be "user-database" or "datadog-web-app". Services
	// will be inherited from parents, so only set this in your app's
	// top level span.
	Service string `json:"service"`

	// Resource is a query to a service. A web application might use
	// resources like "/user/{user_id}". A sql database might use resources
	// like "select * from user where id = ?".
	//
	// You can track thousands of resources (not millions or billions) so
	// prefer normalized resources like "/user/{id}" to "/user/123".
	//
	// Resources should only be set on an app's top level spans.
	Resource string `json:"resource"`

	Type     string             `json:"type"`              // protocol associated with the span
	Start    int64              `json:"start"`             // span start time expressed in nanoseconds since epoch
	Duration int64              `json:"duration"`          // duration of the span expressed in nanoseconds
	Meta     map[string]string  `json:"meta,omitempty"`    // arbitrary map of metadata
	Metrics  map[string]float64 `json:"metrics,omitempty"` // arbitrary map of numeric metrics
	SpanID   uint64             `json:"span_id"`           // identifier of this span
	TraceID  uint64             `json:"trace_id"`          // identifier of the root span
	ParentID uint64             `json:"parent_id"`         // identifier of the span's direct parent
	Error    int32              `json:"error"`             // error status of the span; 0 means no errors
	Sampled  bool               `json:"-"`                 // if this span is sampled (and should be kept/recorded) or not

	sync.RWMutex
	tracer   *Tracer // the tracer that generated this span
	finished bool    // true if the span has been submitted to a tracer.

	// parent contains a link to the parent. In most cases, ParentID can be inferred from this.
	// However, ParentID can technically be overridden (typical usage: distributed tracing)
	// and also, parent == nil is used to identify root and top-level ("local root") spans.
	parent  *Span
	buffer  *spanBuffer
	context *spanContext
}

// newSpan creates a new span. This is a low-level function, required for testing and advanced usage.
// Most of the time one should prefer the Tracer NewRootSpan or NewChildSpan methods.
func newSpan(name, service, resource string, spanID, traceID, parentID uint64, tracer *Tracer) *Span {
	return &Span{
		Name:     name,
		Service:  service,
		Resource: resource,
		Meta:     map[string]string{},
		Metrics:  map[string]float64{},
		SpanID:   spanID,
		TraceID:  traceID,
		ParentID: parentID,
		Start:    now(),
		Sampled:  true,
		tracer:   tracer,
	}
}

// setMeta adds an arbitrary meta field to the current Span. The span
// must be locked outside of this function
func (s *Span) setMeta(key, value string) {
	// We don't lock spans when flushing, so we could have a data race when
	// modifying a span as it's being flushed. This protects us against that
	// race, since spans are marked `finished` before we flush them.
	if s.finished {
		return
	}
	if s.Meta == nil {
		s.Meta = make(map[string]string)
	}
	s.Meta[key] = value

}

// SetMeta adds an arbitrary meta field to the current Span.
// If the Span has been finished, it will not be modified by the method.
func (s *Span) SetMeta(key, value string) {
	s.Lock()
	defer s.Unlock()

	s.setMeta(key, value)

}

// SetMetas adds arbitrary meta fields from a given map to the current Span.
// If the Span has been finished, it will not be modified by the method.
func (s *Span) SetMetas(metas map[string]string) {
	for k, v := range metas {
		s.SetMeta(k, v)
	}
}

// GetMeta will return the value for the given tag or the empty string if it
// doesn't exist.
func (s *Span) GetMeta(key string) string {
	s.RLock()
	defer s.RUnlock()
	if s.Meta == nil {
		return ""
	}
	return s.Meta[key]
}

// SetMetrics adds a metric field to the current Span.
// DEPRECATED: Use SetMetric
func (s *Span) SetMetrics(key string, value float64) {
	s.SetMetric(key, value)
}

// SetMetric sets a float64 value for the given key. It acts
// like `set_meta()` and it simply add a tag without further processing.
// This method doesn't create a Datadog metric.
func (s *Span) SetMetric(key string, val float64) {
	s.Lock()
	defer s.Unlock()

	// We don't lock spans when flushing, so we could have a data race when
	// modifying a span as it's being flushed. This protects us against that
	// race, since spans are marked `finished` before we flush them.
	if s.finished {
		return
	}

	if s.Metrics == nil {
		s.Metrics = make(map[string]float64)
	}
	s.Metrics[key] = val
}

// SetError stores an error object within the span meta. The Error status is
// updated and the error.Error() string is included with a default meta key.
// If the Span has been finished, it will not be modified by this method.
func (s *Span) SetError(err error) {
	if err == nil || s == nil {
		return
	}

	s.Lock()
	defer s.Unlock()
	// We don't lock spans when flushing, so we could have a data race when
	// modifying a span as it's being flushed. This protects us against that
	// race, since spans are marked `finished` before we flush them.
	if s.finished {
		return
	}
	s.Error = 1

	s.setMeta(errorMsgKey, err.Error())
	s.setMeta(errorTypeKey, reflect.TypeOf(err).String())
	stack := debug.Stack()
	s.setMeta(errorStackKey, string(stack))
}

// Finish closes this Span (but not its children) providing the duration
// of this part of the tracing session. This method is idempotent so
// calling this method multiple times is safe and doesn't update the
// current Span. Once a Span has been finished, methods that modify the Span
// will become no-ops.
func (s *Span) Finish() {
	s.finish(now())
}

// FinishWithTime closes this Span at the given `finishTime`. The
// behavior is the same as `Finish()`.
func (s *Span) FinishWithTime(finishTime int64) {
	s.finish(finishTime)
}

func (s *Span) finish(finishTime int64) {
	s.Lock()
	if s.finished {
		// already finished
		return
	}
	if s.Duration == 0 {
		s.Duration = finishTime - s.Start
	}
	s.finished = true
	s.Unlock()

	if s.buffer == nil {
		if s.tracer != nil {
			s.tracer.channels.pushErr(&errorNoSpanBuf{SpanName: s.Name})
		}
		return
	}

	// If not sampled, drop it
	if !s.Sampled {
		return
	}

	s.buffer.AckFinish() // put data in channel only if trace is completely finished

	// It's important that when Finish() exits, the data is put in
	// the channel for real, when the trace is finished.
	// Otherwise, tests could become flaky (because you never know in what state
	// the channel is).
}

// FinishWithErr marks a span finished and sets the given error if it's
// non-nil.
func (s *Span) FinishWithErr(err error) {
	s.SetError(err)
	s.Finish()
}

// String returns a human readable representation of the span. Not for
// production, just debugging.
func (s *Span) String() string {
	lines := []string{
		fmt.Sprintf("Name: %s", s.Name),
		fmt.Sprintf("Service: %s", s.Service),
		fmt.Sprintf("Resource: %s", s.Resource),
		fmt.Sprintf("TraceID: %d", s.TraceID),
		fmt.Sprintf("SpanID: %d", s.SpanID),
		fmt.Sprintf("ParentID: %d", s.ParentID),
		fmt.Sprintf("Start: %s", time.Unix(0, s.Start)),
		fmt.Sprintf("Duration: %s", time.Duration(s.Duration)),
		fmt.Sprintf("Error: %d", s.Error),
		fmt.Sprintf("Type: %s", s.Type),
		"Tags:",
	}

	s.RLock()
	for key, val := range s.Meta {
		lines = append(lines, fmt.Sprintf("\t%s:%s", key, val))

	}
	s.RUnlock()

	return strings.Join(lines, "\n")
}

// SetSamplingPriority sets the sampling priority.
func (s *Span) SetSamplingPriority(priority int) {
	s.SetMetric(samplingPriorityKey, float64(priority))
}

// HasSamplingPriority returns true if sampling priority is set.
// It can be defined to either zero or non-zero.
func (s *Span) HasSamplingPriority() bool {
	_, hasSamplingPriority := s.Metrics[samplingPriorityKey]
	return hasSamplingPriority
}

// GetSamplingPriority gets the sampling priority.
func (s *Span) GetSamplingPriority() int {
	return int(s.Metrics[samplingPriorityKey])
}
