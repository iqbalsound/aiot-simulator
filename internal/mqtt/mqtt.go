// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

// Package mqtt is a small, dependency-free MQTT 3.1.1 broker and client used by
// the AIoT Simulator. It implements the subset required by AI-4301 Labs 5, 10, 11:
// CONNECT (with username/password), PUBLISH (QoS 0/1/2 inbound, retained),
// SUBSCRIBE/UNSUBSCRIBE with + and # wildcards, PING and DISCONNECT.
// Simplification (v0.0.1): messages are delivered to subscribers at QoS 0.
package mqtt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// Message is a published application message.
type Message struct {
	Topic   string
	Payload []byte
	QoS     byte
	Retain  bool
	From    string // client id, or "sim" for internal publishers
	User    string
}

// CONNACK return codes.
const (
	ConnAccepted      = 0
	ConnBadProtocol   = 1
	ConnBadCredential = 4
	ConnNotAuthorized = 5
)

// Broker is an embeddable MQTT broker.
type Broker struct {
	mu       sync.RWMutex
	ln       net.Listener
	sessions map[*session]struct{}
	retained map[string]Message
	subs     map[int]*internalSub
	nextSub  int

	// Auth returns a CONNACK code. hasUser=false means an anonymous connection.
	Auth func(clientID, user, pass string, hasUser bool) byte
	// OnPublish is called for every message accepted by the broker.
	OnPublish func(Message)
	// OnEvent reports connection lifecycle events for the UI.
	OnEvent func(kind, detail string)
}

type internalSub struct {
	filter string
	fn     func(Message)
}

type session struct {
	conn net.Conn
	id   string
	user string
	wmu  sync.Mutex
	smu  sync.Mutex
	subs map[string]byte
}

func NewBroker() *Broker {
	return &Broker{sessions: map[*session]struct{}{}, retained: map[string]Message{}, subs: map[int]*internalSub{}}
}

func (b *Broker) event(kind, detail string) {
	if b.OnEvent != nil {
		b.OnEvent(kind, detail)
	}
}

// Listen starts accepting connections on addr (e.g. "0.0.0.0:1883").
func (b *Broker) Listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.ln = ln
	b.mu.Unlock()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go b.serve(c)
		}
	}()
	return nil
}

// Addr returns the listening address.
func (b *Broker) Addr() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.ln == nil {
		return ""
	}
	return b.ln.Addr().String()
}

// Close stops the listener and disconnects all clients.
func (b *Broker) Close() {
	b.mu.Lock()
	if b.ln != nil {
		b.ln.Close()
		b.ln = nil
	}
	for s := range b.sessions {
		s.conn.Close()
	}
	b.mu.Unlock()
}

// ClientCount returns the number of connected network clients.
func (b *Broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.sessions)
}

// Subscribe registers an in-process subscriber.
func (b *Broker) Subscribe(filter string, fn func(Message)) (cancel func()) {
	b.mu.Lock()
	b.nextSub++
	id := b.nextSub
	b.subs[id] = &internalSub{filter, fn}
	b.mu.Unlock()
	return func() { b.mu.Lock(); delete(b.subs, id); b.mu.Unlock() }
}

// Publish injects a message from inside the simulator (no authentication).
func (b *Broker) Publish(topic string, payload []byte, retain bool) {
	b.route(Message{Topic: topic, Payload: payload, Retain: retain, From: "sim"})
}

func (b *Broker) route(m Message) {
	if m.Retain {
		b.mu.Lock()
		if len(m.Payload) == 0 {
			delete(b.retained, m.Topic)
		} else {
			b.retained[m.Topic] = m
		}
		b.mu.Unlock()
	}
	if b.OnPublish != nil {
		b.OnPublish(m)
	}
	b.mu.RLock()
	var targets []*session
	for s := range b.sessions {
		s.smu.Lock()
		for f := range s.subs {
			if TopicMatch(f, m.Topic) {
				targets = append(targets, s)
				break
			}
		}
		s.smu.Unlock()
	}
	var fns []func(Message)
	for _, is := range b.subs {
		if TopicMatch(is.filter, m.Topic) {
			fns = append(fns, is.fn)
		}
	}
	b.mu.RUnlock()
	for _, s := range targets {
		s.sendPublish(m.Topic, m.Payload, false)
	}
	for _, fn := range fns {
		fn(m)
	}
}

// TopicMatch implements MQTT topic filter matching.
func TopicMatch(filter, topic string) bool {
	if strings.HasPrefix(topic, "$") && (strings.HasPrefix(filter, "#") || strings.HasPrefix(filter, "+")) {
		return false
	}
	f := strings.Split(filter, "/")
	t := strings.Split(topic, "/")
	for i, seg := range f {
		if seg == "#" {
			return true
		}
		if i >= len(t) {
			return false
		}
		if seg != "+" && seg != t[i] {
			return false
		}
	}
	return len(f) == len(t)
}

// ---- wire helpers ----

func readPacket(r *bufio.Reader) (typ byte, flags byte, body []byte, err error) {
	h, err := r.ReadByte()
	if err != nil {
		return
	}
	typ, flags = h>>4, h&0x0f
	mult, n := 1, 0
	for i := 0; i < 4; i++ {
		var d byte
		d, err = r.ReadByte()
		if err != nil {
			return
		}
		n += int(d&127) * mult
		if d&128 == 0 {
			break
		}
		mult *= 128
		if i == 3 {
			err = errors.New("malformed remaining length")
			return
		}
	}
	if n > 1<<20 {
		err = errors.New("packet too large")
		return
	}
	body = make([]byte, n)
	_, err = io.ReadFull(r, body)
	return
}

func encodeLen(n int) []byte {
	var out []byte
	for {
		d := byte(n % 128)
		n /= 128
		if n > 0 {
			d |= 128
		}
		out = append(out, d)
		if n == 0 {
			return out
		}
	}
}

func packet(typ, flags byte, body []byte) []byte {
	p := []byte{typ<<4 | flags}
	p = append(p, encodeLen(len(body))...)
	return append(p, body...)
}

func str(b []byte) (string, []byte, error) {
	if len(b) < 2 {
		return "", nil, errors.New("short string")
	}
	n := int(b[0])<<8 | int(b[1])
	if len(b) < 2+n {
		return "", nil, errors.New("short string")
	}
	return string(b[2 : 2+n]), b[2+n:], nil
}

func mstr(s string) []byte { return append([]byte{byte(len(s) >> 8), byte(len(s))}, s...) }

func (s *session) write(p []byte) {
	s.wmu.Lock()
	s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	s.conn.Write(p)
	s.wmu.Unlock()
}

func (s *session) sendPublish(topic string, payload []byte, retain bool) {
	var fl byte
	if retain {
		fl = 1
	}
	s.write(packet(3, fl, append(mstr(topic), payload...)))
}

func (b *Broker) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	typ, _, body, err := readPacket(r)
	if err != nil || typ != 1 {
		return
	}
	proto, rest, err := str(body)
	if err != nil || len(rest) < 4 {
		return
	}
	level, cflags := rest[0], rest[1]
	keep := int(rest[2])<<8 | int(rest[3])
	rest = rest[4:]
	connack := func(code byte) { c.Write(packet(2, 0, []byte{0, code})) }
	if !((proto == "MQTT" && level == 4) || (proto == "MQIsdp" && level == 3)) {
		connack(ConnBadProtocol)
		return
	}
	cid, rest, err := str(rest)
	if err != nil {
		return
	}
	if cflags&0x04 != 0 { // will topic + will message
		var e error
		if _, rest, e = str(rest); e != nil {
			return
		}
		if _, rest, e = str(rest); e != nil {
			return
		}
	}
	var user, pass string
	hasUser := cflags&0x80 != 0
	if hasUser {
		if user, rest, err = str(rest); err != nil {
			return
		}
	}
	if cflags&0x40 != 0 {
		if pass, _, err = str(rest); err != nil {
			return
		}
	}
	code := byte(ConnAccepted)
	if b.Auth != nil {
		code = b.Auth(cid, user, pass, hasUser)
	}
	if code != ConnAccepted {
		connack(code)
		who := user
		if !hasUser {
			who = "anonymous"
		}
		b.event("refused", fmt.Sprintf("client %q (%s) refused, CONNACK code %d", cid, who, code))
		return
	}
	connack(ConnAccepted)
	s := &session{conn: c, id: cid, user: user, subs: map[string]byte{}}
	b.mu.Lock()
	b.sessions[s] = struct{}{}
	b.mu.Unlock()
	b.event("connect", fmt.Sprintf("client %q connected", cid))
	defer func() {
		b.mu.Lock()
		delete(b.sessions, s)
		b.mu.Unlock()
		b.event("disconnect", fmt.Sprintf("client %q disconnected", cid))
	}()
	for {
		if keep > 0 {
			c.SetReadDeadline(time.Now().Add(time.Duration(keep*3/2) * time.Second))
		} else {
			c.SetReadDeadline(time.Time{})
		}
		typ, fl, body, err := readPacket(r)
		if err != nil {
			return
		}
		switch typ {
		case 3: // PUBLISH
			qos := (fl >> 1) & 3
			topic, rest, err := str(body)
			if err != nil {
				return
			}
			var pid []byte
			if qos > 0 {
				if len(rest) < 2 {
					return
				}
				pid, rest = rest[:2], rest[2:]
			}
			switch qos {
			case 1:
				s.write(packet(4, 0, pid))
			case 2:
				s.write(packet(5, 0, pid))
			}
			b.route(Message{Topic: topic, Payload: append([]byte(nil), rest...), QoS: qos, Retain: fl&1 != 0, From: cid, User: user})
		case 6: // PUBREL
			if len(body) >= 2 {
				s.write(packet(7, 0, body[:2]))
			}
		case 8: // SUBSCRIBE
			if len(body) < 2 {
				return
			}
			pid, rest := body[:2], body[2:]
			ack := append([]byte{}, pid...)
			var newFilters []string
			for len(rest) > 0 {
				f, r2, err := str(rest)
				if err != nil || len(r2) < 1 {
					return
				}
				rest = r2[1:]
				s.smu.Lock()
				s.subs[f] = 0
				s.smu.Unlock()
				newFilters = append(newFilters, f)
				ack = append(ack, 0) // granted QoS 0
			}
			s.write(packet(9, 0, ack))
			b.mu.RLock()
			var ret []Message
			for _, m := range b.retained {
				for _, f := range newFilters {
					if TopicMatch(f, m.Topic) {
						ret = append(ret, m)
						break
					}
				}
			}
			b.mu.RUnlock()
			for _, m := range ret {
				s.sendPublish(m.Topic, m.Payload, true)
			}
		case 10: // UNSUBSCRIBE
			if len(body) < 2 {
				return
			}
			pid, rest := body[:2], body[2:]
			for len(rest) > 0 {
				f, r2, err := str(rest)
				if err != nil {
					return
				}
				s.smu.Lock()
				delete(s.subs, f)
				s.smu.Unlock()
				rest = r2
			}
			s.write(packet(11, 0, pid))
		case 12:
			s.write(packet(13, 0, nil))
		case 14:
			return
		}
	}
}

// ---- client ----

// Client is a minimal MQTT 3.1.1 client (used by the simulator's own publishers
// and by the UI's "MQTT test client").
type Client struct {
	conn net.Conn
	r    *bufio.Reader
	mu   sync.Mutex
	pid  uint16
	acks chan uint16
	// OnMessage is called for every message received. Set it before Subscribe.
	OnMessage func(topic string, payload []byte)
	closed    chan struct{}
}

// ErrRefused wraps a non-zero CONNACK code.
type ErrRefused struct{ Code byte }

func (e ErrRefused) Error() string {
	switch e.Code {
	case ConnBadProtocol:
		return "connection refused: unacceptable protocol version (code 1)"
	case ConnBadCredential:
		return "connection refused: bad user name or password (code 4)"
	case ConnNotAuthorized:
		return "connection refused: not authorised (code 5)"
	}
	return fmt.Sprintf("connection refused (code %d)", e.Code)
}

// Dial connects and performs the MQTT handshake.
func Dial(addr, clientID, user, pass string, hasUser bool) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return nil, err
	}
	var cf byte = 0x02
	body := append(mstr("MQTT"), 4)
	if hasUser {
		cf |= 0x80 | 0x40
	}
	body = append(body, cf, 0, 30)
	body = append(body, mstr(clientID)...)
	if hasUser {
		body = append(body, mstr(user)...)
		body = append(body, mstr(pass)...)
	}
	conn.Write(packet(1, 0, body))
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	r := bufio.NewReader(conn)
	typ, _, b, err := readPacket(r)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if typ != 2 || len(b) < 2 {
		conn.Close()
		return nil, errors.New("unexpected reply to CONNECT")
	}
	if b[1] != 0 {
		conn.Close()
		return nil, ErrRefused{b[1]}
	}
	conn.SetReadDeadline(time.Time{})
	c := &Client{conn: conn, r: r, acks: make(chan uint16, 16), closed: make(chan struct{})}
	go c.loop()
	return c, nil
}

func (c *Client) loop() {
	defer close(c.closed)
	for {
		typ, fl, body, err := readPacket(c.r)
		if err != nil {
			return
		}
		switch typ {
		case 3:
			t, rest, err := str(body)
			if err != nil {
				return
			}
			if (fl>>1)&3 > 0 && len(rest) >= 2 {
				rest = rest[2:]
			}
			if c.OnMessage != nil {
				c.OnMessage(t, rest)
			}
		case 4, 9, 7:
			if len(body) >= 2 {
				select {
				case c.acks <- uint16(body[0])<<8 | uint16(body[1]):
				default:
				}
			}
		}
	}
}

func (c *Client) nextID() uint16 { c.mu.Lock(); c.pid++; id := c.pid; c.mu.Unlock(); return id }

func (c *Client) waitAck(id uint16) error {
	t := time.After(3 * time.Second)
	for {
		select {
		case a := <-c.acks:
			if a == id {
				return nil
			}
		case <-t:
			return errors.New("timeout waiting for acknowledgement")
		case <-c.closed:
			return errors.New("connection closed")
		}
	}
}

// Publish sends a message; QoS 1 waits for PUBACK.
func (c *Client) Publish(topic string, payload []byte, qos byte, retain bool) error {
	var fl = qos << 1
	if retain {
		fl |= 1
	}
	body := mstr(topic)
	var id uint16
	if qos > 0 {
		id = c.nextID()
		body = append(body, byte(id>>8), byte(id))
	}
	body = append(body, payload...)
	c.mu.Lock()
	_, err := c.conn.Write(packet(3, fl, body))
	c.mu.Unlock()
	if err != nil {
		return err
	}
	if qos == 1 {
		return c.waitAck(id)
	}
	return nil
}

// Subscribe registers filters and waits for SUBACK.
func (c *Client) Subscribe(filters ...string) error {
	id := c.nextID()
	body := []byte{byte(id >> 8), byte(id)}
	for _, f := range filters {
		body = append(body, mstr(f)...)
		body = append(body, 0)
	}
	c.mu.Lock()
	_, err := c.conn.Write(packet(8, 2, body))
	c.mu.Unlock()
	if err != nil {
		return err
	}
	return c.waitAck(id)
}

// Close sends DISCONNECT and closes the socket.
func (c *Client) Close() {
	c.mu.Lock()
	c.conn.Write(packet(14, 0, nil))
	c.mu.Unlock()
	c.conn.Close()
}
