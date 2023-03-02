/*
Copyright © 2023 Jonathan Gotti <jgotti at jgotti dot org>
SPDX-FileType: SOURCE
SPDX-License-Identifier: MIT
SPDX-FileCopyrightText: 2023 Jonathan Gotti <jgotti@jgotti.org>
*/

package jobExecutor

import (
	"strings"
)

// Map indexes correspond to job index in the queue
type JobsError map[int]error

func (es JobsError) Error() string {
	return es.String()
}
func (es JobsError) String() string {
	var strs []string
	for _, err := range es {
		strs = append(strs, err.Error())
	}
	return strings.Join(strs, "\n")
}
