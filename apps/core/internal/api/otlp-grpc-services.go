package api

import (
	"context"
	"orion/core/internal/logstore"
	"orion/core/internal/metricstore"
	"orion/core/internal/tracestore"
	"strconv"
	"time"

	collogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collmetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/mem"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/tap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

// otlpGRPCAppKey carries the authenticated app from the tap handler to the Export handlers.
type otlpGRPCAppKey struct{}

// authenticateOTLPGRPC runs for every call as soon as its headers arrive, before the message is read,
// so a caller without a valid ingest key never makes Core allocate or decode a request. The key travels
// as `authorization: Bearer <key>` metadata, the same value OTLP/HTTP takes in its Authorization header.
func (s *Server) authenticateOTLPGRPC(ctx context.Context, info *tap.Info) (context.Context, error) {
	authorization := ""
	if values := info.Header.Get("authorization"); len(values) > 0 {
		authorization = values[0]
	}
	app, failure := s.authenticateIngest(authorization)
	if failure != nil {
		return nil, otlpGRPCStatus(context.Background(), failure, "")
	}
	return context.WithValue(ctx, otlpGRPCAppKey{}, app), nil
}

// recoverOTLPGRPC keeps a panic in one export from taking Core down; gin does the same for HTTP.
func (s *Server) recoverOTLPGRPC(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("OTLP/gRPC handler panicked", "method", info.FullMethod, "panic", r)
			err = status.Error(codes.Internal, "Internal error")
		}
	}()
	return handler(ctx, req)
}

func otlpGRPCApp(ctx context.Context) (*logstore.AppContext, error) {
	if app, ok := ctx.Value(otlpGRPCAppKey{}).(*logstore.AppContext); ok && app != nil {
		return app, nil
	}
	return nil, status.Error(codes.Unauthenticated, "Valid ingest key required")
}

// registerOTLPGRPCServices registers the OTLP logs, metrics and trace collector services.
func (s *Server) registerOTLPGRPCServices(srv *grpc.Server) {
	registerOTLPService(srv, &collogs.LogsService_ServiceDesc, &otlpGRPCLogs{server: s})
	registerOTLPService(srv, &collmetrics.MetricsService_ServiceDesc, &otlpGRPCMetrics{server: s})
	registerOTLPService(srv, &coltrace.TraceService_ServiceDesc, &otlpGRPCTraces{server: s})
}

// registerOTLPService registers a service whose requests are decoded here instead of by grpc-go. grpc-go
// answers a message that does not parse with Internal, which clients read as a server fault and may retry;
// it is the client's mistake, so it answers InvalidArgument. (grpc-go has already sent its own status by
// the time a handler could change it, so the decode itself has to move.)
func registerOTLPService(srv *grpc.Server, original *grpc.ServiceDesc, implementation any) {
	desc := *original
	desc.Methods = make([]grpc.MethodDesc, len(original.Methods))
	for i, method := range original.Methods {
		handler := method.Handler
		desc.Methods[i] = grpc.MethodDesc{
			MethodName: method.MethodName,
			Handler: func(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				return handler(service, ctx, func(message any) (err error) {
					// This decode runs before the server's panic-recovery interceptor, so it recovers on its own.
					defer func() {
						if recover() != nil {
							err = status.Error(codes.InvalidArgument, "Request is not a valid OTLP export request")
						}
					}()
					var raw otlpRawRequest
					if err := decode(&raw); err != nil {
						return err // too large, bad compression and the like keep grpc-go's own status
					}
					if err := proto.Unmarshal(raw.data, message.(proto.Message)); err != nil {
						return status.Error(codes.InvalidArgument, "Request is not a valid OTLP export request")
					}
					return nil
				}, interceptor)
			},
		}
	}
	srv.RegisterService(&desc, implementation)
}

// otlpRawRequest receives the bytes of a request, after decompression and the size check, unparsed.
type otlpRawRequest struct{ data []byte }

// otlpWireCodec is the server's codec: protobuf as usual, except that an otlpRawRequest just keeps the bytes.
type otlpWireCodec struct{ protobuf encoding.CodecV2 }

func newOTLPWireCodec() otlpWireCodec { return otlpWireCodec{protobuf: encoding.GetCodecV2("proto")} }

func (c otlpWireCodec) Marshal(v any) (mem.BufferSlice, error) { return c.protobuf.Marshal(v) }

func (c otlpWireCodec) Unmarshal(data mem.BufferSlice, v any) error {
	if raw, ok := v.(*otlpRawRequest); ok {
		raw.data = data.Materialize()
		return nil
	}
	return c.protobuf.Unmarshal(data, v)
}

func (otlpWireCodec) Name() string { return "proto" }

type otlpGRPCLogs struct {
	collogs.UnimplementedLogsServiceServer
	server *Server
}

func (h *otlpGRPCLogs) Export(ctx context.Context, request *collogs.ExportLogsServiceRequest) (*collogs.ExportLogsServiceResponse, error) {
	app, err := otlpGRPCApp(ctx)
	if err != nil {
		return nil, err
	}
	result, failure := h.server.exportLogs(app, request)
	if failure != nil {
		return nil, h.server.otlpGRPCError(ctx, failure, "log records")
	}
	return logstore.OTLPResponseMessage(result.Rejected, result.Errors), nil
}

type otlpGRPCMetrics struct {
	collmetrics.UnimplementedMetricsServiceServer
	server *Server
}

func (h *otlpGRPCMetrics) Export(ctx context.Context, request *collmetrics.ExportMetricsServiceRequest) (*collmetrics.ExportMetricsServiceResponse, error) {
	app, err := otlpGRPCApp(ctx)
	if err != nil {
		return nil, err
	}
	result, failure := h.server.exportMetrics(app, request)
	if failure != nil {
		return nil, h.server.otlpGRPCError(ctx, failure, "data points")
	}
	return metricstore.OTLPMetricsResponseMessage(result.Rejected, result.Message), nil
}

type otlpGRPCTraces struct {
	coltrace.UnimplementedTraceServiceServer
	server *Server
}

func (h *otlpGRPCTraces) Export(ctx context.Context, request *coltrace.ExportTraceServiceRequest) (*coltrace.ExportTraceServiceResponse, error) {
	app, err := otlpGRPCApp(ctx)
	if err != nil {
		return nil, err
	}
	result, failure := h.server.exportTraces(app, request)
	if failure != nil {
		return nil, h.server.otlpGRPCError(ctx, failure, "spans")
	}
	return tracestore.OTLPResponseMessage(result.Rejected, result.Errors), nil
}

// otlpGRPCError turns a refused request into a gRPC error. Failures worth retrying carry the wait as a
// `retry-after` trailer (whole seconds, as the HTTP header) and as the standard RetryInfo detail, which
// OpenTelemetry exporters honour; a request that can never succeed does not, so it is not retried.
func (s *Server) otlpGRPCError(ctx context.Context, failure *ingestFailure, noun string) error {
	if failure.kind == failInternal {
		s.logger.Error("OTLP/gRPC ingest failed", "message", failure.message, "error", failure.err)
	}
	return otlpGRPCStatus(ctx, failure, noun)
}

// otlpGRPCStatus maps a failure to its gRPC status: Unauthenticated for a bad key, InvalidArgument for a
// request that cannot be read or carries too much, ResourceExhausted for a spent rate limit or a full
// write queue, Unavailable when the signal is off or storage is full, Internal otherwise.
func otlpGRPCStatus(ctx context.Context, failure *ingestFailure, noun string) error {
	switch failure.kind {
	case failUnauthenticated:
		return status.Error(codes.Unauthenticated, "Valid ingest key required")
	case failUnavailable:
		return status.Error(codes.Unavailable, "This signal is not enabled on this Core instance")
	case failTooMany:
		return status.Error(codes.InvalidArgument, "Too many "+noun+" in one request (limit 10000); send smaller batches")
	case failInvalid:
		return status.Error(codes.InvalidArgument, "Request is not a valid OTLP export request")
	case failRateLimited:
		return retryableStatus(ctx, codes.ResourceExhausted, "Ingest rate limit exceeded; retry after the indicated time", failure)
	case failQueueFull:
		return retryableStatus(ctx, codes.ResourceExhausted, "Write queue is full; retry shortly", failure)
	case failStorageFull:
		return retryableStatus(ctx, codes.Unavailable, "Storage is full; the data was not accepted. Free disk space or lower retention.", failure)
	}
	return status.Error(codes.Internal, "Internal error")
}

func retryableStatus(ctx context.Context, code codes.Code, message string, failure *ingestFailure) error {
	seconds := retryAfterSeconds(failure.retryAfter)
	_ = grpc.SetTrailer(ctx, metadata.Pairs("retry-after", strconv.Itoa(seconds)))
	st := status.New(code, message)
	if detailed, err := st.WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(time.Duration(seconds) * time.Second)}); err == nil {
		st = detailed
	}
	return st.Err()
}
