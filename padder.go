// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"bytes"
	"fmt"
	"net/http"
)

type paddingType int

const (
	none   paddingType = iota
	length
	script

	scriptStart  = "<html><body>"
	// TODO(hochhaus): make Sprintf calls safe to XSS attacks.
	scriptDomain = "<script>try{document.domain='%s';}catch(e){}</script>\n"
	scriptMsg    = "<script>try{parent.m(%s)}catch(e){}</script>\n"
	scriptEnd    = "<script>try{parent.d();}catch(e){}</script>\n"
)

type padder struct {
	w http.ResponseWriter
	f http.Flusher
	t paddingType
}

func guessType(r *http.Request) paddingType {
	if r.FormValue("TYPE") == "html" {
		return script
	}
	return length
}

func newPadder(w http.ResponseWriter, r *http.Request) (*padder, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		panic("webserver doesn't support flushing")
	}
	t := guessType(r)
	p := &padder{w, f, t}
	switch p.t {
	case script:
		p.w.Header().Set("Content-Type", "text/html; charset=utf-8")
		payload := scriptStart
		if domain := r.FormValue("DOMAIN"); domain != "" {
			payload += fmt.Sprintf(scriptDomain, domain)
		}
		payload += IEPadding
		_, err := p.w.Write([]byte(payload))
		if err != nil {
			return nil, err
		}
	default:
		p.w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		p.w.WriteHeader(http.StatusOK)
	}
	p.f.Flush()
	return p, nil
}

func (p padder) chunk(b []byte) error {
	switch p.t {
	case script:
		// TODO(hochhaus): sanitize b as "JsonString"
		p.w.Write([]byte(fmt.Sprintf(scriptMsg, b)))
	case length:
		utf8Length := len(bytes.Runes(b))
		p.w.Write([]byte(fmt.Sprintf("%d\n", utf8Length)))
		p.w.Write(b)
	default:
		p.w.Write(b)
	}
	p.f.Flush()
	return nil
}

func (p padder) end() error {
	if p.t == script {
		_, err := p.w.Write([]byte(scriptEnd))
		if err != nil {
			return err
		}
		p.f.Flush()
	}
	// TODO(hochhaus): ensure that handle func returns (to force end chunk)
	return nil
}
