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
// http.Flusher interface.
type GZIPResponseWriter struct {
	*gzip.Writer
	http.ResponseWriter
	browserGZIP, detectDone bool
	buf                     bytes.Buffer
}

func (w GZIPResponseWriter) detectAndWriteBuffer() {
	if w.detectDone {
		return
	}

	// Detect Content-Type if not explicitly set
	header := w.ResponseWriter.Header()
	if _, haveType := header["Content-Type"]; !haveType {
		header.Set("Content-Type", http.DetectContentType(w.buf.Bytes()))
	}

	// Check for uncompressed content. Only uncompressed output should be gzipped
	uncompressedType := strings.HasPrefix(header.Get("Content-Type"), "text/")
	compressCandidate := uncompressedType && w.buf.Len() >= minGZIPSize
	if compressCandidate {
		header.Set("Vary", "accept-encoding")
	}

	// Write buffer
	if w.browserGZIP && compressCandidate {
		header.Set("Content-Encoding", "gzip")
		w.Writer = gzip.NewWriter(w.ResponseWriter)
		w.Writer.Write(w.buf.Bytes())
	} else {
		w.ResponseWriter.Write(w.buf.Bytes())
	}
	w.detectDone = true
}

func (w GZIPResponseWriter) Write(b []byte) (int, error) {
	if !w.detectDone {
		// write to buffer
		l, err := w.buf.Write(b)
		if err != nil {
			return l, err
		}
		if w.buf.Len() < sniffLen {
			return l, nil
		}
		w.detectAndWriteBuffer()
		return l, nil
	}

	if w.Writer != nil {
		return w.Writer.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// Flush writes any buffered data to the underlying ResponseWriter.
func (w GZIPResponseWriter) Flush() {
	w.detectAndWriteBuffer()
	if w.Writer != nil {
		w.Writer.Flush()
		// TODO(ahochhaus): is w.ResponseWriter.(http.Flusher).Flush() necessary?
	} else {
		w.ResponseWriter.(http.Flusher).Flush()
	}
}

// Close cleans up the underlying gzip.Writer (if necessary).
func (w GZIPResponseWriter) Close() {
	w.detectAndWriteBuffer()
	if w.Writer != nil {
		w.Writer.Close()
	}
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
