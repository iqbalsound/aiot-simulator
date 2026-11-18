// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.

package mqtt

import (
	"errors"
	"testing"
	"time"
)

func TestTopicMatch(t *testing.T) {
	cases := []struct {
		f, t string
		ok   bool
	}{
		{"aiot/lab5/#", "aiot/lab5/temperature", true},
		{"aiot/lab5/#", "aiot/lab5", true},
		{"aiot/+/temperature", "aiot/lab5/temperature", true},
		{"aiot/+/temperature", "aiot/lab5/humidity", false},
		{"aiot/lab5", "aiot/lab5/x", false},
		{"#", "$SYS/x", false},
		{"v1/devices/me/telemetry", "v1/devices/me/telemetry", true},
	}
	for _, c := range cases {
		if TopicMatch(c.f, c.t) != c.ok {
			t.Errorf("TopicMatch(%q,%q) want %v", c.f, c.t, c.ok)
		}
	}
}

func TestPubSubAuthRetain(t *testing.T) {
	b := NewBroker()
	secure := false
	b.Auth = func(id, u, p string, has bool) byte {
		if !secure {
			return ConnAccepted
		}
		if has && u == "student" && p == "pw" {
			return ConnAccepted
		}
		if !has {
			return ConnNotAuthorized
		}
		return ConnBadCredential
	}
	if err := b.Listen("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	addr := b.Addr()

	got := make(chan string, 4)
	sub, err := Dial(addr, "sub", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	sub.OnMessage = func(tp string, p []byte) { got <- tp + "=" + string(p) }
	if err := sub.Subscribe("aiot/lab5/#"); err != nil {
		t.Fatal(err)
	}
	pub, err := Dial(addr, "pub", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := pub.Publish("aiot/lab5/temperature", []byte("24.5"), 1, false); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-got:
		if m != "aiot/lab5/temperature=24.5" {
			t.Fatalf("got %s", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no message delivered")
	}
	// retained message + late subscriber
	pub.Publish("aiot/lab5/status", []byte("online"), 0, true)
	time.Sleep(100 * time.Millisecond)
	late, _ := Dial(addr, "late", "", "", false)
	lg := make(chan string, 2)
	late.OnMessage = func(tp string, p []byte) { lg <- tp + "=" + string(p) }
	late.Subscribe("aiot/#")
	select {
	case m := <-lg:
		if m != "aiot/lab5/status=online" {
			t.Fatalf("retained got %s", m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("retained not delivered")
	}
	// internal subscriber + internal publish
	ig := make(chan Message, 1)
	cancel := b.Subscribe("aiot/lab11/anomaly", func(m Message) { ig <- m })
	b.Publish("aiot/lab11/anomaly", []byte("1"), false)
	select {
	case m := <-ig:
		if string(m.Payload) != "1" {
			t.Fatal("bad payload")
		}
	case <-time.After(time.Second):
		t.Fatal("internal sub missed")
	}
	cancel()
	// authentication
	secure = true
	_, err = Dial(addr, "anon", "", "", false)
	var ref ErrRefused
	if !errors.As(err, &ref) || ref.Code != ConnNotAuthorized {
		t.Fatalf("anonymous should be refused with 5, got %v", err)
	}
	_, err = Dial(addr, "bad", "student", "wrong", true)
	if !errors.As(err, &ref) || ref.Code != ConnBadCredential {
		t.Fatalf("bad password should be refused with 4, got %v", err)
	}
	ok, err := Dial(addr, "ok", "student", "pw", true)
	if err != nil {
		t.Fatal(err)
	}
	ok.Close()
}
