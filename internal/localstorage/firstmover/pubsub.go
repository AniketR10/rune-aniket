// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package firstmover

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/ernestrc/go-multierror"
	logd "github.com/ernestrc/logd-go/logging"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"unstable.build/rune/internal/debug"
	"unstable.build/rune/internal/localstorage/firstmover/pubsubpb"
)

// important it's not 0 to not lose messages
// when re-connecting to another leader
const clientStreamBuffer = 100

var errAlreadySubscribed = errors.New("this client is already subscribed to this topic")

type pubsub struct {
	pubsubpb.UnimplementedPubSubServer
	mu       sync.Locker
	id       string
	pid      string
	lockFile string
	ctx      context.Context
	cancelFn func()
	readyCtx context.Context
	ready    func()
	closed   bool
	// pubReadyCtx gates the server Publish handler until this
	// incarnation has restored the subscriptions recovered from its
	// predecessor. Without it a publish retried across a leader
	// handoff can land on the new leader before resubscribe runs,
	// find zero subscribers and vacuously succeed, silently dropping
	// an at-least-once message.
	pubReadyCtx context.Context
	pubReady    func()

	// leader only
	subscribers    map[string][]*subscriber
	leaderConn     *grpc.ClientConn
	leaderListener net.Listener

	// follower only
	followerConn *grpc.ClientConn

	// leader and follower
	client        pubsubpb.PubSubClient
	clientStreams map[string]chan msgError // stream cache
}

type subscriber struct {
	errors chan error
	stream pubsubpb.PubSub_ReceiveServer
	mu     sync.Mutex
}

func (p *pubsub) init(lockFile, pid string) {
	p.cancelFn = func() {}
	p.readyCtx, p.ready = context.WithCancel(context.Background())
	p.pubReadyCtx, p.pubReady = context.WithCancel(context.Background())
	p.lockFile = lockFile
	p.pid = pid
}

func (p *pubsub) reset() {
	p.clientStreams = make(map[string]chan msgError)
	p.subscribers = make(map[string][]*subscriber)
	p.id = uuid.New().String()
	p.cancelFn()
	p.ctx, p.cancelFn = context.WithCancel(context.Background())
	p.closed = false
}

// initLeader assumes this pubsub server has already been registered.
// This cannot be done here because RegisterPubSubServer cannot be
// called after Serve is called, but the dial below will always fail
// if Serve has not been called yet.
func (p *pubsub) initLeader(
	_ context.Context, _ *grpc.Server, listener net.Listener,
) error {
	_ = p.Close()
	p.reset()
	p.ready()

	// to massively simplify streams implementation,
	// publish/subscribe as a leader also act as a client of the pubsub server.
	addr := listener.Addr()
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	opts = append(opts, grpc.WithContextDialer(
		func(ctx context.Context, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, addr.Network(), addr.String())
		},
	))
	conn, err := grpc.NewClient("passthrough:///", opts...)
	if err != nil {
		return fmt.Errorf("dial leader server: %v", err)
	}

	p.client = pubsubpb.NewPubSubClient(conn)
	p.leaderConn = conn
	p.leaderListener = listener
	p.log(log.DebugLevel, "initialized pubsub instance as leader")
	return nil
}

func (p *pubsub) initFollower(conn *grpc.ClientConn) {
	_ = p.Close()
	p.reset()
	p.client = pubsubpb.NewPubSubClient(conn)
	p.followerConn = conn
	p.log(log.DebugLevel, "initialized pubsub instance as follower")
}

func (p *pubsub) publish(
	ctx context.Context,
	topic string, msg []byte,
) error {
	p.mu.Lock()
	client := p.client
	p.mu.Unlock()
	req := pubsubpb.PublishRequest{
		Topic:  topic,
		Data:   msg,
		Sender: p.id,
	}
	p.log(log.TraceLevel, "client is publishing message %q to topic %q", msg, topic)
	_, err := client.Publish(ctx, &req)
	if err != nil {
		return err
	}

	return nil
}

type msgError struct {
	msg *pubsubpb.ReceiveMessage_Data
	err error
}

func (p *pubsub) subscribe(
	subscriptionCtx context.Context, topic string,
	excl bool,
) (context.Context, chan msgError, error) {
	p.mu.Lock()
	quitCtx := p.ctx
	stream, ok := p.clientStreams[topic]
	client := p.client
	p.mu.Unlock()
	if ok {
		if excl {
			return quitCtx, nil, errAlreadySubscribed
		}
		return quitCtx, stream, nil
	}

	req := pubsubpb.ReceiveMessage_Request{
		Topic: topic,
	}
	// if topic stream doesn't exist, create a new one
	msg := pubsubpb.ReceiveMessage{
		Req: &req,
	}
	pbStream, err := client.Receive(subscriptionCtx)
	if err != nil {
		return quitCtx, nil, err
	}
	err = pbStream.Send(&msg)
	if err != nil {
		return quitCtx, nil, err
	}
	// blocks until server has sent header and so
	// connection is fully established
	md, err := pbStream.Header()
	if err != nil {
		return quitCtx, nil, err
	}

	p.log(log.DebugLevel, "subscribed to topic %s, metadata: %+v", topic, md)

	p.mu.Lock()
	select {
	case <-quitCtx.Done():
		p.mu.Unlock()
		return quitCtx, nil, quitCtx.Err()
	case <-subscriptionCtx.Done():
		p.mu.Unlock()
		return quitCtx, nil, subscriptionCtx.Err()
	default:
	}
	stream = make(chan msgError, clientStreamBuffer)
	p.clientStreams[topic] = stream
	p.mu.Unlock()

	// stream messages until until srv stream is done,
	// broken or pubsub is closed
	go debug.CapturePanicReport(func() {

		defer func() {
			p.mu.Lock()
			defer p.mu.Unlock()

			if s, ok := p.clientStreams[topic]; ok && s == stream {
				// allow re-connect
				delete(p.clientStreams, topic)
			}
			close(stream)
		}()

		for {
			msg, err := pbStream.Recv()
			p.log(log.TraceLevel, "client stream Recv returned: %q, %v", msg, err)
			select {
			case stream <- msgError{msg: msg.GetData(), err: err}:
				if err != nil {
					return
				}
				p.log(log.TraceLevel, "client stream sending ack for message: %q", msg)
				ack := pubsubpb.ReceiveMessage_Ack{}
				ackMsg := pubsubpb.ReceiveMessage{Ack: &ack}
				err = pbStream.SendMsg(&ackMsg)
				if err != nil {
					p.log(log.TraceLevel, "client stream error sending ack for message: %q: %v",
						msg, err)
					select {
					case stream <- msgError{err: err}:
					case <-quitCtx.Done():
						return
					}
				}
			case <-quitCtx.Done():
				p.log(log.TraceLevel, "ignoring message %q, err=%v: quit context is done",
					msg, err)
				// best effort
				select {
				case stream <- msgError{err: err}:
				default:
				}
				return
			case <-subscriptionCtx.Done():
				return
			}
			if err != nil {
				return
			}
		}

	})

	return quitCtx, stream, nil
}

func (p *pubsub) receive(
	ctx context.Context, topic string,
) ([]byte, error) {
	// do not use the receive context as the
	// subscripion context for a subscription
	// created here: a client could set a Receive timeout
	// and unintentionally cancel the stream.
	p.mu.Lock()
	subscribeCtx := p.ctx
	p.mu.Unlock()
	quitCtx, stream, err := p.subscribe(subscribeCtx, topic, false)
	if err != nil {
		return nil, err
	}

	for {
		p.log(log.TraceLevel, "client is waiting to receive a message for topic %q", topic)
		var msgErr msgError
		var ok bool
		select {
		case msgErr, ok = <-stream:
		case <-quitCtx.Done():
			return nil, quitCtx.Err()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if !ok {
			return nil, status.Errorf(codes.Aborted, "closed")
		}
		if msgErr.err != nil {
			return nil, msgErr.err
		}
		// when we publish as a leader, we should not receive
		// these messages on the next call to Receive.
		if msgErr.msg.GetSender() == p.id {
			p.log(log.TraceLevel, "ignoring self published message %q for topic %q",
				msgErr.msg, topic)
			continue
		}
		return msgErr.msg.GetData(), nil
	}
}

func (p *pubsub) Publish(
	ctx context.Context, req *pubsubpb.PublishRequest,
) (*pubsubpb.PublishResponse, error) {
	<-p.readyCtx.Done()

	p.mu.Lock()
	pubReadyCtx := p.pubReadyCtx
	p.mu.Unlock()
	select {
	case <-pubReadyCtx.Done():
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	topic := req.GetTopic()
	msg := req.GetData()

	p.mu.Lock()
	subscribers := p.subscribers[topic]
	if p.closed {
		p.mu.Unlock()
		return nil, status.Errorf(codes.Aborted, "closed")
	}

	p.log(log.TraceLevel, "server is broadcasting message %q for topic %q: subscribers: %d",
		msg, topic, len(subscribers))

	errors := make([]error, len(subscribers))
	var wg sync.WaitGroup
	wg.Add(len(subscribers))
	for i, sub := range subscribers {
		go debug.CapturePanicReport(func() {
			func(sub *subscriber, i int) {
				defer wg.Done()

				sub.mu.Lock()
				defer sub.mu.Unlock()

				var req pubsubpb.ReceiveMessage
				var data pubsubpb.ReceiveMessage_Data
				data.Data = msg
				req.Data = &data
				if serr := sub.stream.Send(&req); serr != nil {
					// cancel offending stream, but also return
					// an error to this rpc so we provide at least once semantics
					select {
					case sub.errors <- fmt.Errorf("stream send data: %w", serr):
					default:
					}

					errors[i] = serr
					return
				}

				var ack pubsubpb.ReceiveMessage
				if serr := sub.stream.RecvMsg(&ack); serr != nil {
					select {
					case sub.errors <- fmt.Errorf("stream receive ack: %w", serr):
					default:
					}
					errors[i] = serr
					return
				}

				if ack.GetAck() == nil {
					err := fmt.Errorf("protocol error: received non ack: %+v", &ack)
					select {
					case sub.errors <- err:
					default:
					}
					errors[i] = err
				}
			}(sub, i)
		})
	}

	p.mu.Unlock()
	wg.Wait()

	p.log(log.TraceLevel, "server is done broadcasting message %q for topic %q to %d subscribers",
		msg, topic, len(subscribers))

	var err error
	for _, serr := range errors {
		if serr != nil {
			err = multierror.Append(err, serr)
		}
	}
	if err != nil {
		return nil, err
	}
	return new(pubsubpb.PublishResponse), nil
}

func (p *pubsub) Receive(srv pubsubpb.PubSub_ReceiveServer) error {
	<-p.readyCtx.Done()

	msg, err := srv.Recv()
	if err != nil {
		return err
	}

	topic := msg.GetReq().GetTopic()
	if topic == "" {
		return errors.New("empty topic in receive request")
	}

	errors := make(chan error)
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return status.Errorf(codes.Aborted, "closed")
	}
	sub := &subscriber{errors: errors, stream: srv}
	sub.mu.Lock()
	p.subscribers[topic] = append(p.subscribers[topic], sub)
	quitCtx := p.ctx
	p.mu.Unlock()

	// header is waited upon by client to guarantee that
	// the subscriber will be found, if Receive is followed by fast
	// subsequent calls to Publish.
	md := metadata.New(make(map[string]string))
	md.Append("ID", p.id)

	err = srv.SendHeader(md)
	sub.mu.Unlock()
	if err != nil {
		return fmt.Errorf("rpc send header: %w", err)
	}

	// remove ch when this receive stream is done
	defer func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		for i, c := range p.subscribers[topic] {
			if c.errors == errors {
				copy(p.subscribers[topic][i:], p.subscribers[topic][i+1:])
				p.subscribers[topic] = p.subscribers[topic][:len(p.subscribers[topic])-1]
				return
			}
		}
	}()

	select {
	case err := <-errors:
		return err
	case <-quitCtx.Done():
		return quitCtx.Err()
	case <-srv.Context().Done():
		return nil
	}
}

func (p *pubsub) Close() (err error) {
	if p.closed {
		return
	}

	if p.cancelFn != nil {
		defer p.cancelFn()
	}

	if p.leaderConn != nil {
		if cerr := p.leaderConn.Close(); err != nil {
			err = multierror.Append(err, cerr)
		}
		p.leaderConn = nil
	}
	if p.leaderListener != nil {
		if cerr := p.leaderListener.Close(); err != nil {
			err = multierror.Append(err, cerr)
		}
		p.leaderListener = nil
	}
	if p.followerConn != nil {
		if cerr := p.followerConn.Close(); err != nil {
			err = multierror.Append(err, cerr)
		}
		p.followerConn = nil
	}

	p.subscribers = nil
	p.clientStreams = nil
	p.closed = true

	return err
}

func (s *pubsub) log(level log.Level, msg string, args ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithFields(log.Fields{
		logd.KeyClass: "firstmover.pubsub",
		"address":     fmt.Sprintf("%p", s),
		"lock":        s.lockFile,
		"pid":         s.pid,
	}).Logf(level, msg, args...)
}
