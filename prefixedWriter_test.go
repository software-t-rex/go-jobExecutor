/*
Copyright © 2023 Jonathan Gotti <jgotti at jgotti dot org>
SPDX-FileType: SOURCE
SPDX-License-Identifier: MIT
SPDX-FileCopyrightText: 2023 Jonathan Gotti <jgotti@jgotti.org>
*/

package jobExecutor

import (
	"bytes"
	"testing"
)

func TestNewPrefixedWriter(t *testing.T) {
	type args struct {
		prefix string
		in     string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"Should prefix a given line", args{"TESTING: ", "test line\n"}, "TESTING: test line\n"},
		{"Should prefix a all lines when multiple lines", args{"TESTING: ", "test line\ntest line2\n"}, "TESTING: test line\nTESTING: test line2\n"},
		{"Should ensure new line on input end", args{"TESTING: ", "test line"}, "TESTING: test line\n"},
		{"Should ensure new line on input end", args{"TESTING: ", "test line\ntest line2"}, "TESTING: test line\nTESTING: test line2\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			pw := NewPrefixedWriter(buf, tt.args.prefix)
			pw.Write([]byte(tt.args.in))
			if buf.String() != tt.want {
				t.Errorf("Write = %v, want %v", buf, tt.want)
			}
		})
	}
}
