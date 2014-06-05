// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"bytes"
	"net/http"
	"testing"
)

const (
	goldBufferedProxy   = `111112`
	goldBufferedProxyIE = `<html><body><script>try{document.domain='example.com'}catch(e){}</script>7cca69475363026330a0d99468e88d23ce95e222591126443015f5f462d9a177186c8701fb45a6ffee0daf1a178fc0f58cd309308fba7e6f011ac38c9cdd4580760f1d4560a84d5ca0355ecbbed2ab715a3350fe0c479050640bd0e77acec90c58c4d3dd0f5cf8d4510e68c8b12e087bd88cad349aafd2ab16b07b0b1b8276091217a44a9fe92fedacffff48092ee693af
<script>try{parent.m('11111')}catch(e){}</script>
<script>try{parent.m('2')}catch(e){}</script>
<script>try{parent.d()}catch(e){}</script>`
	goldMessages = `54
[[0,["c","23sd..32","b",8]],[1,["appMsg1","appMsg2"]]]`
	goldMessagesIE = `<html><body>7cca69475363026330a0d99468e88d23ce95e222591126443015f5f462d9a177186c8701fb45a6ffee0daf1a178fc0f58cd309308fba7e6f011ac38c9cdd4580760f1d4560a84d5ca0355ecbbed2ab715a3350fe0c479050640bd0e77acec90c58c4d3dd0f5cf8d4510e68c8b12e087bd88cad349aafd2ab16b07b0b1b8276091217a44a9fe92fedacffff48092ee693af
<script>try{parent.m('[[0,[\x22c\x22,\x2223sd..32\x22,\x22b\x22,8]],[1,[\x22appMsg1\x22,\x22appMsg2\x22]]]')}catch(e){}</script>
<script>try{parent.d()}catch(e){}</script>`
)

type mockResponse struct {
	head http.Header
	buf  bytes.Buffer
	raw  []byte
}

func newMockResponse() *mockResponse {
	return &mockResponse{head: make(http.Header)}
}

func (w *mockResponse) Header() http.Header {
	return w.head
}

func (w *mockResponse) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return len(b), nil
}

func (w *mockResponse) WriteHeader(int) {
	panic("not implemented")
}

func (w *mockResponse) Flush() {
	w.raw = w.buf.Bytes()
}

func newMockRequest(method, urlStr string) *http.Request {
	r, err := http.NewRequest(method, urlStr, bytes.NewBufferString(""))
	if err != nil {
		panic(err)
	}
	return r
}

func TestBufferedProxy(t *testing.T) {
	r := newMockRequest("GET", "/channel/test?TYPE=xmlhttp")
	w := newMockResponse()
	p := newPadder(w, r)
	if p.t == length {
		p.t = none
	}
	p.chunk("11111")
	p.chunk("2")
	p.end()
	if !bytes.Equal(w.raw, []byte(goldBufferedProxy)) {
		t.Errorf("Found %s, want %s", w.raw, goldBufferedProxy)
	}
}

func TestBufferedProxyIE(t *testing.T) {
	r := newMockRequest("GET", "/channel/test?TYPE=html&DOMAIN=example.com")
	w := newMockResponse()
	p := newPadder(w, r)
	if p.t == length {
		p.t = none
	}
	p.chunk("11111")
	p.chunk("2")
	p.end()
	if !bytes.Equal(w.raw, []byte(goldBufferedProxyIE)) {
		t.Errorf("Found %s, want %s", w.raw, goldBufferedProxyIE)
	}
}

func TestMessages(t *testing.T) {
	r := newMockRequest("GET", "/channel?TYPE=xmlhttp")
	w := newMockResponse()
	p := newPadder(w, r)
	msgs := []*Message{
		&Message{0, []byte(jsonArray([]interface{}{"c", "23sd..32", "b", 8}))},
		&Message{1, []byte(jsonArray([]interface{}{"appMsg1", "appMsg2"}))},
	}
	p.chunkMessages(msgs)
	p.end()
	if !bytes.Equal(w.raw, []byte(goldMessages)) {
		t.Errorf("Found %s, want %s", w.raw, goldMessages)
	}
}

func TestMessagesIE(t *testing.T) {
	r := newMockRequest("GET", "/channel?TYPE=html")
	w := newMockResponse()
	p := newPadder(w, r)
	msgs := []*Message{
		&Message{0, []byte(jsonArray([]interface{}{"c", "23sd..32", "b", 8}))},
		&Message{1, []byte(jsonArray([]interface{}{"appMsg1", "appMsg2"}))},
	}
	p.chunkMessages(msgs)
	p.end()
	if !bytes.Equal(w.raw, []byte(goldMessagesIE)) {
		t.Errorf("Found %s, want %s", w.raw, goldMessagesIE)
	}
}
