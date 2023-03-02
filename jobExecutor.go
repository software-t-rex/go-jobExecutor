/*
Copyright © 2023 Jonathan Gotti <jgotti at jgotti dot org>
SPDX-FileType: SOURCE
SPDX-License-Identifier: MIT
SPDX-FileCopyrightText: 2023 Jonathan Gotti <jgotti@jgotti.org>
*/

package jobExecutor

import (
	_ "embed"
	"fmt"
	"html/template"
	"os/exec"
	"strings"
)

//go:embed output.gtpl
var dfltTemplateString string
var outputTemplate *template.Template

type jobEventHandler func(jobs JobList, jobId int)
type jobsEventHandler func(jobs JobList)
type JobExecutor struct {
	jobs JobList
	opts *executeOptions
}

func init() {
	SetTemplateString(dfltTemplateString)
}

func indent(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return pad + strings.Replace(v, "\n", "\n"+pad, -1)
}
func trim(v string) string {
	return strings.Trim(v, "\n")
}

// Template for all outputs related to jobs
// It must define the following templates:
//   - startSummary: which will receive a JobList
//   - jobStatus: which will receive a single job
//   - progressReport: which will receive a jobList
//   - doneReport: which will receive a jobList
func SetTemplateString(templateString string) {
	outputTemplate = template.Must(template.New("executor-output").
		Funcs(template.FuncMap{
			"indent": indent,
			"trim":   trim,
		}).
		Parse(templateString),
	)
}

func augmentJobHandler(fn jobEventHandler, decoratorFn jobEventHandler) jobEventHandler {
	if fn == nil {
		return decoratorFn
	}
	return func(jobs JobList, jobId int) {
		fn(jobs, jobId)
		decoratorFn(jobs, jobId)
	}
}
func augmentJobsHandler(fn jobsEventHandler, decoratorFn jobsEventHandler) jobsEventHandler {
	if fn == nil {
		return decoratorFn
	}
	return func(jobs JobList) {
		fn(jobs)
		decoratorFn(jobs)
	}
}

// Instanciate a new JobExecutor
func NewExecutor() *JobExecutor {
	executor := &JobExecutor{
		opts: &executeOptions{},
	}
	return executor
}

// Return the total number of jobs added to the jobExecutor
func (e *JobExecutor) Len() int {
	return len(e.jobs)
}

// Add multiple job command to run
func (e *JobExecutor) AddJobCmds(cmdsAndArgs ...[]string) *JobExecutor {
	for _, cmdAndArgs := range cmdsAndArgs {
		e.AddJobCmd(cmdAndArgs[0], cmdAndArgs[1:]...)
	}
	return e
}

// Add a single job command to run
func (e *JobExecutor) AddJobCmd(cmd string, args ...string) *JobExecutor {
	e.jobs = append(e.jobs, &job{Cmd: exec.Command(cmd, args...)})
	return e
}

// Add one or more job function to run (func() (string, error))
func (e *JobExecutor) AddJobFns(fns ...runnableFn) *JobExecutor {
	for _, fn := range fns {
		e.jobs = append(e.jobs, &job{Fn: fn})
	}
	return e
}

// Add a handler which will be called after a job is terminated
func (e *JobExecutor) OnJobDone(fn jobEventHandler) *JobExecutor {
	e.opts.onJobDone = augmentJobHandler(e.opts.onJobDone, fn)
	return e
}

// Add a handler which will be called after all jobs are terminated
func (e *JobExecutor) OnJobsDone(fn jobsEventHandler) *JobExecutor {
	e.opts.onJobsDone = augmentJobsHandler(e.opts.onJobsDone, fn)
	return e
}

// Add a handler which will be called before a job is started
func (e *JobExecutor) OnJobStart(fn jobEventHandler) *JobExecutor {
	e.opts.onJobStart = augmentJobHandler(e.opts.onJobStart, fn)
	return e
}

// Add a handler which will be called before any jobs is started
func (e *JobExecutor) OnJobsStart(fn jobsEventHandler) *JobExecutor {
	e.opts.onJobsStart = augmentJobsHandler(e.opts.onJobsStart, fn)
	return e
}

// Output a summary of jobs that will be run
func (e *JobExecutor) WithStartSummary() *JobExecutor {
	e.opts.onJobsStart = func(jobs JobList) {
		fmt.Print(jobs.execTemplate("startSummary"))
	}
	return e
}

// Output a line to say a job is starting
func (e *JobExecutor) WithStartOutput() *JobExecutor {
	e.opts.onJobStart = func(jobs JobList, jobId int) {
		fmt.Print("Starting " + jobs[jobId].execTemplate("jobStatusLine"))
	}
	return e
}

// Display full jobStatus as they arrive
func (e *JobExecutor) WithFifoOutput() *JobExecutor {
	e.opts.onJobDone = func(jobs JobList, jobId int) {
		fmt.Print(jobs[jobId].execTemplate("jobStatusFull"))
	}
	return e
}

// Display doneReport when all jobs are Done
func (e *JobExecutor) WithOrderedOutput() *JobExecutor {
	e.opts.onJobsDone = func(jobs JobList) {
		fmt.Print(jobs.execTemplate("doneReport"))
	}
	return e
}

// Display a job status report updated each time a job start or end
// be carefull when dealing with other handler that generate output
// as it will potentially break progress output
func (e *JobExecutor) WithProgressOutput() *JobExecutor {
	e.opts.onJobsStart = augmentJobsHandler(e.opts.onJobsStart, func(jobs JobList) {
		fmt.Print(jobs.execTemplate("startProgressReport"))
	})
	printProgress := func(jobs JobList, jobId int) {
		esc := fmt.Sprintf("\033[%dA", len(e.jobs)) // clean sequence
		fmt.Print(esc + jobs.execTemplate("progressReport"))
	}
	e.opts.onJobDone = augmentJobHandler(e.opts.onJobDone, printProgress)
	e.opts.onJobStart = augmentJobHandler(e.opts.onJobStart, printProgress)
	return e
}

// Effectively execute jobs and return collected errors as JobsError
func (e *JobExecutor) Execute() JobsError {
	var errs = make([]error, e.Len())
	var res = make(JobsError, e.Len())
	e.OnJobDone(func(jobs JobList, jobId int) {
		err := jobs[jobId].Err
		if err != nil {
			errs[jobId] = err
		}
	})
	execute(e.jobs, *e.opts)
	for jobId, err := range errs {
		if err != nil {
			res[jobId] = err
		}
	}
	return res
}
