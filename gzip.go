// Copyright (c) 2014 SameGoal LLC. All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wc

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strings"
)

const (
	minGZIPSize = 250
	sniffLen    = 512
)

// GZIPResponseWriter wraps a http.ResponseWriter and provides optional
// gzip compression. Streaming HTTP chunks is supported using the
// http.Flusher interface. Close notification is supported via the
// http.CloseNotifier interface.
type GZIPResponseWriter struct {
	*gzip.Writer
	http.ResponseWriter
	browserGZIP, detectDone bool
	buf                     bytes.Buffer
}

func (w *GZIPResponseWriter) detect(isFlush bool) {
	if w.detectDone {
		return
	}

	// Detect Content-Type if not explicitly set
	header := w.ResponseWriter.Header()
	if _, haveType := header["Content-Type"]; !haveType {
		header.Set("Content-Type", http.DetectContentType(w.buf.Bytes()))
	}

	// Check for uncompressed content. Only uncompressed output should be gzipped
	uncompType := strings.HasPrefix(header.Get("Content-Type"), "text/") ||
		header.Get("Content-Type") == "application/javascript"
	compressCandidate := uncompType && (isFlush || w.buf.Len() >= minGZIPSize)
	if compressCandidate {
		header.Set("Vary", "accept-encoding")
	}

	// Setup gzip Writer
	if w.browserGZIP && compressCandidate {
		header.Set("Content-Encoding", "gzip")
		w.Writer = gzip.NewWriter(w.ResponseWriter)
	}
	w.detectDone = true
}

func (w *GZIPResponseWriter) writeBuffer() {
	if w.buf.Len() == 0 {
		return
	}

	// Write buffer
	if w.Writer != nil {
		w.Writer.Write(w.buf.Bytes())
	} else {
		w.ResponseWriter.Write(w.buf.Bytes())
	}
	w.buf.Truncate(0)
}

// Header return the http.Header from the underlying http.ResponseWriter.
func (w *GZIPResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// Writes the response to the underlying http.ResponseWriter while
// transparently supporting compression for uncompressed data.
func (w *GZIPResponseWriter) Write(b []byte) (int, error) {
	if !w.detectDone {
		// write to buffer
		l, err := w.buf.Write(b)
		if err != nil {
			return l, err
		}
		if w.buf.Len() < sniffLen {
			return l, nil
		}
		w.detect(false)
		w.writeBuffer()
		return l, nil
	}

	w.writeBuffer()
	if w.Writer != nil {
		return w.Writer.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// WriteHeader detects gzip compession and then proxies the supplied status
// code to the underlying http.ResponseWriter.
func (w *GZIPResponseWriter) WriteHeader(code int) {
	// Note, w.buf will be empty when WriteHeader is called explicitly by the
	// application. Do not forceCompress as most non-200 responses are small.
	w.detect(false)
	w.ResponseWriter.WriteHeader(code)
}

// Flush writes any buffered data to the underlying ResponseWriter.
func (w *GZIPResponseWriter) Flush() {
	w.detect(true)
	w.writeBuffer()
	if w.Writer != nil {
		w.Writer.Flush()
	}
	w.ResponseWriter.(http.Flusher).Flush()
}

// Close cleans up the underlying gzip.Writer (if necessary).
func (w *GZIPResponseWriter) Close() {
	w.detect(false)
	w.writeBuffer()
	if w.Writer != nil {
		w.Writer.Close()
	}
}

// CloseNotify return the underlying CloseNotify() channel.
func (w *GZIPResponseWriter) CloseNotify() <-chan bool {
	return w.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

// NewGZIPResponseWriter creates a new GZIPResponseWriter. The http.Request is
// necessary to check if the browser supports GZIP compression and will not be
// used after the call to NewGZIPResponseWriter returns.
func NewGZIPResponseWriter(
	w http.ResponseWriter,
	r *http.Request,
) *GZIPResponseWriter {
	browserGZIP := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
	return &GZIPResponseWriter{ResponseWriter: w, browserGZIP: browserGZIP}
}
