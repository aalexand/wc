// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
)

type paddingType int

const (
	none paddingType = iota
	length
	script
)

var (
	scriptStart = template.Must(template.New("").Parse("<html><body>" +
		"{{if .Domain}}<script>try{document.domain='{{.Domain}}'}catch(e){}" +
		"</script>{{end}}\n7cca69475363026330a0d99468e88d23ce95e22259112644301" +
		"5f5f462d9a177186c8701fb45a6ffee0daf1a178fc0f58cd309308fba7e6f011ac38c9c" +
		"dd4580760f1d4560a84d5ca0355ecbbed2ab715a3350fe0c479050640bd0e77acec90c5" +
		"8c4d3dd0f5cf8d4510e68c8b12e087bd88cad349aafd2ab16b07b0b1b8276091217a44a" +
		"9fe92fedacffff48092ee693af\n"))
	scriptMessage = template.Must(template.New("").Parse(
		"<script>try{parent.m('{{.Message}}')}catch(e){}</script>\n"))
	scriptEnd = template.Must(template.New("").Parse(
		"<script>try{parent.d()}catch(e){}</script>\n"))

	xmlhttpMessage = template.Must(template.New("").Parse(
		"{{.UTF8Length}}\n{{.Message}}"))
)

type padder struct {
	w      http.ResponseWriter
	f      http.Flusher
	t      paddingType
	setup  bool
	domain string
}

type startData struct {
	Domain string
}

type messageData struct {
	Message string
}

type xmlhttpMessageData struct {
	UTF8Length int
	Message    []byte
}

func jsonArray(vals ...interface{}) []byte {
	replyJSON, err := json.Marshal(vals)
	if err != nil {
		panic(err)
	}
	return replyJSON
}

func jsonObject(vals map[string]interface{}) []byte {
	replyJSON, err := json.Marshal(vals)
	if err != nil {
		panic(err)
	}
	return replyJSON
}

func guessType(r *http.Request) paddingType {
	if r.FormValue("TYPE") == "html" {
		return script
	}
	return length
}

func newPadder(w http.ResponseWriter, r *http.Request) *padder {
	f, ok := w.(http.Flusher)
	if !ok {
		panic("webserver doesn't support flushing")
	}
	t := guessType(r)
	return &padder{w, f, t, false, r.FormValue("DOMAIN")}
}

func (p *padder) start() error {
	p.setup = true
	header := p.w.Header()
	// All WebChannel traffic must not be cached by the browser or proxies
	header.Set("Expires", "Fri, 01 Jan 1990 00:00:00 GMT")
	header.Set("Cache-Control", "max-age=0, must-revalidate, private")
	// X-Content-Type-Options is required on Chrome for incremental
	// XMLHttpRequest HTTP chunk processing with Content-Type text/plain.
	header.Set("X-Content-Type-Options", "nosniff")
	switch p.t {
	case script:
		header.Set("Content-Type", "text/html; charset=utf-8")
		d := startData{p.domain}
		if err := scriptStart.Execute(p.w, d); err != nil {
			return err
		}
	default:
		header.Set("Content-Type", "text/plain; charset=utf-8")
	}
	return nil
}

func (p *padder) chunk(b []byte) error {
	if !p.setup {
		if err := p.start(); err != nil {
			return err
		}
	}
	switch p.t {
	case script:
		d := messageData{string(b)}
		if err := scriptMessage.Execute(p.w, d); err != nil {
			return err
		}
	case length:
		utf8Length := len(bytes.Runes(b))
		d := xmlhttpMessageData{utf8Length, b}
		if err := xmlhttpMessage.Execute(p.w, d); err != nil {
			return err
		}
	default:
		if _, err := p.w.Write(b); err != nil {
			return err
		}
	}
	p.f.Flush()
	return nil
}

func (p *padder) end() error {
	if !p.setup {
		if err := p.start(); err != nil {
			return err
		}
	}
	if p.t == script {
		d := struct{}{}
		if err := scriptEnd.Execute(p.w, d); err != nil {
			return err
		}
		p.f.Flush()
	}
	return nil
}
