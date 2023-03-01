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
	opts *ExecuteOptions
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
// it must define the following templates:
// - startSummary: which will receive a JobList
// - jobStatus: which will receive a single job
// - progressReport: which will receive a jobList
// - doneReport: which will receive a jobList
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

// instanciate a new JobExecutor
func NewExecutor() *JobExecutor {
	executor := &JobExecutor{
		opts: &ExecuteOptions{},
	}
	return executor
}

// return the total number of jobs in the pool
func (e *JobExecutor) Len() int {
	return len(e.jobs)
}

// Add miltiple job command to execute
func (e *JobExecutor) AddJobCmds(cmdsAndArgs ...[]string) *JobExecutor {
	for _, cmdAndArgs := range cmdsAndArgs {
		e.AddJobCmd(cmdAndArgs[0], cmdAndArgs[1:]...)
	}
	return e
}
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

// Add an handler which will be call after a jobs is terminated
func (e *JobExecutor) OnJobDone(fn jobEventHandler) *JobExecutor {
	e.opts.onJobDone = augmentJobHandler(e.opts.onJobDone, fn)
	return e
}

// Add an handler which will be call after all jobs are terminated
func (e *JobExecutor) OnJobsDone(fn jobsEventHandler) *JobExecutor {
	e.opts.onJobsDone = augmentJobsHandler(e.opts.onJobsDone, fn)
	return e
}

// Add an handler which will be call before a jobs is started
func (e *JobExecutor) OnJobStart(fn jobEventHandler) *JobExecutor {
	e.opts.onJobStart = augmentJobHandler(e.opts.onJobStart, fn)
	return e
}

// Add an handler which will be call before any jobs is started
func (e *JobExecutor) OnJobsStart(fn jobsEventHandler) *JobExecutor {
	e.opts.onJobsStart = augmentJobsHandler(e.opts.onJobsStart, fn)
	return e
}

// Output a summary of the job that will be run
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

// display doneReport when all jobs are Done
func (e *JobExecutor) WithOrderedOutput() *JobExecutor {
	e.opts.onJobsDone = func(jobs JobList) {
		fmt.Print(jobs.execTemplate("doneReport"))
	}
	return e
}

// will override onJobStarts / onJobStart / onJobDone handlers previsously defined
// generally you should avoid using these method with other handlers bound to the
// JobExecutor instance
func (e *JobExecutor) WithProgressOutput() *JobExecutor {
	e.opts.onJobsStart = func(jobs JobList) {
		fmt.Print(jobs.execTemplate("startProgressReport"))
	}
	esc := fmt.Sprintf("\033[%dA", len(e.jobs)) // clean sequence
	printProgress := func(jobs JobList, jobId int) { fmt.Print(esc + jobs.execTemplate("progressReport")) }
	e.opts.onJobDone = printProgress
	e.opts.onJobStart = printProgress
	return e
}

// effectively execute jobs and return collected errors as JobsError
func (e *JobExecutor) Execute() JobsError {
	var errs = make(JobsError, e.Len())
	e.OnJobDone(func(jobs JobList, jobId int) {
		err := jobs[jobId].Err
		if err != nil {
			errs[jobId] = NewJobError(jobId, err)
		}
	})
	execute(e.jobs, *e.opts)
	return errs
}
