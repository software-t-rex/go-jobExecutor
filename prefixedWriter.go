/*
Copyright © 2023 Jonathan Gotti <jgotti at jgotti dot org>
SPDX-FileType: SOURCE
SPDX-License-Identifier: MIT
SPDX-FileCopyrightText: 2023 Jonathan Gotti <jgotti@jgotti.org>
*/

package jobExecutor

import (
	"io"
	"strings"
)

type prefixedWriter struct {
	prefix string
	buf    io.Writer
}

func NewPrefixedWriter(buf io.Writer, prefix string) *prefixedWriter {
	return &prefixedWriter{prefix: prefix, buf: buf}
}
func (pb *prefixedWriter) replacer(b []byte) []byte {
	bstring := string(b)
	return []byte(pb.prefix + strings.Join(strings.Split(strings.TrimSuffix(bstring, "\n"), "\n"), "\n"+pb.prefix) + "\n")
}
func (b *prefixedWriter) Write(p []byte) (int, error) {
	_, err := b.buf.Write([]byte(b.replacer(p)))
	return len(p), err
}
