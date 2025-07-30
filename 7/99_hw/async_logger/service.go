package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные
type Admin struct {
	UnimplementedAdminServer
	logger *LoggsAndStats
}

func (a *Admin) Logging(n *Nothing, stream Admin_LoggingServer) error {
	ctx := stream.Context()
	a.logger.LogEvent(ctx)
	mt, _ := metadata.FromIncomingContext(ctx)
	consumers := mt.Get("consumer")

	ch := make(chan *Event)

	a.logger.subMu.Lock()
	a.logger.subs[consumers[0]] = ch
	a.logger.subMu.Unlock()

	defer func() {
		a.logger.subMu.Lock()
		delete(a.logger.subs, consumers[0])
		a.logger.subMu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case evt := <-ch:
			if err := stream.Send(evt); err != nil {
				return err
			}
		}
	}
}

func (a *Admin) Statistics(s *StatInterval, stream Admin_StatisticsServer) error {
	ctx := stream.Context()
	a.logger.LogEvent(ctx)
	mt, _ := metadata.FromIncomingContext(ctx)
	consumers := mt.Get("consumer")
	a.logger.statsMu.Lock()
	a.logger.stats[consumers[0]] = &Stats{
		consumers: make(map[string]uint64),
		methods:   make(map[string]uint64),
	}
	a.logger.statsMu.Unlock()

	ticker := time.NewTicker(time.Second * time.Duration(s.IntervalSeconds))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			a.logger.statsMu.Lock()
			delete(a.logger.stats, consumers[0])
			a.logger.statsMu.Unlock()
			return nil
		case <-ticker.C:
			a.logger.statsMu.RLock()
			st := a.logger.stats[consumers[0]]
			a.logger.statsMu.RUnlock()

			st.statMu.Lock()
			byMethod := make(map[string]uint64, len(st.methods))
			for k, v := range st.methods {
				byMethod[k] = v
			}
			byConsumer := make(map[string]uint64, len(st.consumers))
			for k, v := range st.consumers {
				byConsumer[k] = v
			}
			st.methods = make(map[string]uint64)
			st.consumers = make(map[string]uint64)
			st.statMu.Unlock()

			if err := stream.Send(&Stat{
				Timestamp:  time.Now().Unix(),
				ByMethod:   byMethod,
				ByConsumer: byConsumer,
			}); err != nil {
				return err
			}
		}
	}
}

type Biz struct {
	UnimplementedBizServer
	logger *LoggsAndStats
}

func (b *Biz) Check(ctx context.Context, in *Nothing) (*Nothing, error) {
	b.logger.LogEvent(ctx)
	return in, nil
}

func (b *Biz) Add(ctx context.Context, in *Nothing) (*Nothing, error) {
	b.logger.LogEvent(ctx)
	return in, nil
}

func (b *Biz) Test(ctx context.Context, in *Nothing) (*Nothing, error) {
	b.logger.LogEvent(ctx)
	return in, nil
}

type Stats struct {
	statMu    sync.Mutex
	methods   map[string]uint64
	consumers map[string]uint64
}

type LoggsAndStats struct {
	subMu   sync.RWMutex
	subs    map[string]chan *Event
	statsMu sync.RWMutex
	stats   map[string]*Stats
}

func (l *LoggsAndStats) LogEvent(ctx context.Context) {
	md, _ := metadata.FromIncomingContext(ctx)
	consumer := "unknown"
	if c := md.Get("consumer"); len(c) > 0 {
		consumer = c[0]
	}
	method, _ := ctx.Value("method").(string)

	host := "unknown"
	if p, ok := peer.FromContext(ctx); ok {
		host = p.Addr.String()
	}

	evt := &Event{
		Consumer:  consumer,
		Method:    method,
		Host:      host,
		Timestamp: time.Now().Unix(),
	}

	// chans := make([]chan *Event, 0, len(l.subs))
	// for _, ch := range l.subs {
	// 	chans = append(chans, ch)
	// }

	l.subMu.RLock()
	for _, ch := range l.subs {
		ch <- evt
	}
	l.subMu.RUnlock()
	l.statsMu.RLock()
	for _, cStats := range l.stats {
		cStats.statMu.Lock()
		cStats.methods[method] += 1
		cStats.consumers[consumer] += 1
		cStats.statMu.Unlock()
	}
	l.statsMu.RUnlock()
}

func StartMyMicroservice(ctx context.Context, listenAddr string, ACLData string) error {
	var acl map[string][]string
	err := json.Unmarshal([]byte(ACLData), &acl)
	if err != nil {
		return fmt.Errorf("error to unmarshal ACLData: %w", err)
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("error to listen: %w", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(ACLInterceptor(acl)),
		grpc.StreamInterceptor(ACLStreamInterceptor(acl)),
	)
	logger := &LoggsAndStats{
		subs:  make(map[string]chan *Event),
		stats: make(map[string]*Stats),
	}
	RegisterAdminServer(grpcServer, &Admin{logger: logger})
	RegisterBizServer(grpcServer, &Biz{logger: logger})

	go func() {
		grpcServer.Serve(listener)
	}()
	go func() {
		<-ctx.Done()
		grpcServer.Stop()
	}()

	return nil
}

type wrappedServerStream struct {
	grpc.ServerStream
	method string
}

func (w *wrappedServerStream) Context() context.Context {
	return context.WithValue(w.ServerStream.Context(), "method", w.method)
}

func ACLStreamInterceptor(acl map[string][]string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(ss.Context())

		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}

		consumers := md.Get("consumer")

		if len(consumers) == 0 {
			return status.Error(codes.Unauthenticated, "missing consumer")
		}

		consumer, ok := acl[consumers[0]]

		if !ok {
			return status.Error(codes.Unauthenticated, "no rights")
		}

		allow := isAllowedConsumer(consumer, info.FullMethod)
		if !allow {
			return status.Error(codes.Unauthenticated, "no rights")
		}

		ws := &wrappedServerStream{ServerStream: ss, method: info.FullMethod}

		return handler(srv, ws)
	}
}

func ACLInterceptor(acl map[string][]string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		consumers := md.Get("consumer")

		if len(consumers) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing consumer")
		}

		consumer, ok := acl[consumers[0]]

		if !ok {
			return nil, status.Error(codes.Unauthenticated, "no rights")
		}

		allow := isAllowedConsumer(consumer, info.FullMethod)
		if !allow {
			return nil, status.Error(codes.Unauthenticated, "no rights")
		}

		ctx = context.WithValue(ctx, "method", info.FullMethod)

		return handler(ctx, req)
	}
}

func isAllowedConsumer(consumer []string, method string) bool {
	var allow bool
	for _, right := range consumer {
		if str, ok := strings.CutSuffix(right, "/*"); ok && strings.Contains(method, str) || right == method {
			allow = true
			break
		}
	}
	return allow
}
