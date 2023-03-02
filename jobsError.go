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

type JobError struct {
	Id            int
	OriginalError error
}

func NewJobError(jobId int, err error) JobError {
	jobErr := JobError{Id: jobId, OriginalError: err}
	return jobErr
}
func (e *JobError) Error() string {
	return e.OriginalError.Error()
}
func (e *JobError) String() string {
	return e.OriginalError.Error()
}

// Map indexes correspond to job index in the queue
type JobsError map[int]JobError

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
