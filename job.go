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

var ErrRequiredJobFailed = fmt.Errorf("required job failed")
var ErrUndefinedTemplate = fmt.Errorf("template is not defined, see jobExecutor.setTemplate")

type runnableFn func() (string, error)
type JobList []*job
type job struct {
	id          int
	Cmd         *exec.Cmd
	Fn          runnableFn
	displayName string
	Res         string
	Err         error
	status      int
	StartTime   time.Time
	Duration    time.Duration
	DependsOn   []*job
	mutex       sync.RWMutex
}

// ************************** public Job API **************************//

type Job struct {
	job *job
}

type NamedJob struct {
	Name string
	// must be *execCmd or runnableFn
	Job interface{}
}

// return internal job Id, correspond to insertion order in an executor
func (j *Job) Id() int { return j.job.id }

// check the given job is of *exec.Cmd type
func (j *Job) IsCmdJob() bool { return j.job.Cmd != nil }

// check the given job is of func() (string, error) type
func (j *Job) IsFnJob() bool { return j.job.Fn != nil }

// allow to check the status of the job (concurrency safe)
//
//	job.IsState(jobExecutor.JobStateSucceed)
//	job.IsState(jobExecutor.JobStateRunning)
func (j *Job) IsState(state int) bool { return j.job.IsState(state) }

// return the assigned name of a job or a computed one
func (j *Job) Name() string { return j.job.Name() }

// return the combinedOutput of job (only after execution)
// this is concurrency safe
func (j *Job) CombinedOutput() string {
	j.job.mutex.RLock()
	res := j.job.Res
	j.job.mutex.RUnlock()
	return res
}

// return the error returned by a job if any (only after execution)
// this is concurrency safe
func (j *Job) Err() error {
	j.job.mutex.RLock()
	err := j.job.Err
	j.job.mutex.RUnlock()
	return err
}

// ************************** Internam Job API **************************//

func (j *job) run(done func()) {
	defer done()
	j.mutex.RLock()
	dependsOn := j.DependsOn
	j.mutex.RUnlock()
	if len(dependsOn) > 0 {
		hasDepErr := false
		for _, job := range dependsOn {
			if !job.IsState(JobStateSucceed) {
				hasDepErr = true
				break
			}
		}
		if hasDepErr {
			j.mutex.Lock()
			j.Err = ErrRequiredJobFailed
			j.status = JobStateDone | JobStateFailed
			j.Duration = time.Since(j.StartTime)
			j.mutex.Unlock()
			return
		}
	}
	if j.Cmd != nil {
		var res []byte
		var err error
		if j.Cmd.Stderr == nil && j.Cmd.Stdout == nil {
			res, err = j.Cmd.CombinedOutput()
		} else { // don't collect outputs if user already dealt with
			err = j.Cmd.Run()
		}
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

// this methods should not be called once the job executor is running as it is not thread safe
func (j *job) SetDisplayName(name string) {
	j.displayName = name
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
			fmt.Fprintln(os.Stderr, ErrUndefinedTemplate.Error(), r)
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
