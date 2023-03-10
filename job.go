/*
Copyright © 2023 Jonathan Gotti <jgotti at jgotti dot org>
SPDX-FileType: SOURCE
SPDX-License-Identifier: MIT
SPDX-FileCopyrightText: 2023 Jonathan Gotti <jgotti@jgotti.org>
*/

package jobExecutor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"text/template"
	"time"
)

const (
	JobStatePending = 0
	JobStateRunning = 1
	JobStateDone    = 2
	JobStateSucceed = 4
	JobStateFailed  = 8
)

type runnableFn func() (string, error)
type JobList []*job
type job struct {
	Cmd         *exec.Cmd
	Fn          runnableFn
	displayName string
	Res         string
	Err         error
	status      int
	StartTime   time.Time
	Duration    time.Duration
	mutex       sync.RWMutex
}

func (j *job) run(done func()) {
	defer done()
	if j.Cmd != nil {
		res, err := j.Cmd.CombinedOutput()
		j.mutex.Lock()
		j.Res = string(res)
		j.Err = err
	} else if j.Fn != nil {
		res, err := j.Fn()
		j.mutex.Lock()
		j.Res = res
		j.Err = err
	}
	if j.Err != nil {
		j.status = JobStateDone | JobStateFailed
	} else {
		j.status = JobStateDone | JobStateSucceed
	}
	j.Duration = time.Since(j.StartTime)
	j.mutex.Unlock()
}

// Try to return the command string or the function name (using reflect)
func (j *job) Name() string {
	if j == nil {
		return "NotAJob"
	} else if j.displayName != "" {
		return j.displayName
	} else if j.Cmd != nil {
		return strings.Join(j.Cmd.Args, " ")
	} else if j.Fn != nil {
		return runtime.FuncForPC(reflect.ValueOf(j.Fn).Pointer()).Name()
	}
	return "EmptyJob"
}

// Test if a job is in a given JobState
func (j *job) IsState(jobState int) bool {
	j.mutex.RLock()
	var res bool
	if jobState == 0 {
		res = j.status == 0
	} else {
		res = j.status&jobState != 0
	}
	j.mutex.RUnlock()
	return res
}

// Helper method for jobs execTemplate
func tplExec(tpl *template.Template, subject interface{}) string {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "template is not defined, see jobExecutor.setTemplate", r)
		}
	}()
	var out bytes.Buffer
	err := tpl.Execute(&out, subject)
	if err != nil {
		fmt.Fprintln(os.Stderr, tpl, err.Error())
		return ""
	}
	return out.String()
}

func (j *job) execTemplate(tpl *template.Template) string {
	return tplExec(tpl, j)
}
func (jobs *JobList) execTemplate(tpl *template.Template) string {
	return tplExec(tpl, jobs)
}
