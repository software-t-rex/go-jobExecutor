/*
Copyright © 2023 Jonathan Gotti <jgotti at jgotti dot org>
SPDX-FileType: SOURCE
SPDX-License-Identifier: MIT
SPDX-FileCopyrightText: 2023 Jonathan Gotti <jgotti@jgotti.org>
*/
package jobExecutor

import (
	"runtime"
	"sync"
	"time"
)

// never close this channel, it's only purpose is to limit concurrency.
// closing it will make next call to execute cause a panic
var limiterChan chan struct{}

// set the default number of concurrent jobs to run default to GOMAXPROCS
func SetMaxConcurrentJobs(n int) {
	if n < 1 {
		n = runtime.GOMAXPROCS(0)
	}
	limiterChan = make(chan struct{}, n)
}

func init() {
	SetMaxConcurrentJobs(runtime.GOMAXPROCS(0))
}

type executeOptions struct {
	onJobsStart func(jobs JobList)
	onJobStart  func(jobs JobList, jobIndex int)
	onJobDone   func(jobs JobList, jobIndex int)
	onJobsDone  func(jobs JobList)
}

// effectively launch the child process, call on jobDone
// you should prepare child process before by calling either
// PrepareCmds, PrepareFns
// returns the number of errors encountered
// @todo add cancelation support
func execute(jobs JobList, opts executeOptions) {
	if opts.onJobsStart != nil {
		opts.onJobsStart(jobs)
	}
	var wg sync.WaitGroup
	wg.Add(len(jobs))
	for i, child := range jobs {
		limiterChan <- struct{}{}
		jobIndex := i
		job := child
		job.mutex.Lock()
		job.StartTime = time.Now()
		job.status = JobStateRunning
		job.mutex.Unlock()
		if opts.onJobStart != nil {
			opts.onJobStart(jobs, jobIndex)
		}
		go child.run(func() {
			defer func() { <-limiterChan }()
			defer wg.Done()
			if opts.onJobDone != nil {
				opts.onJobDone(jobs, jobIndex)
			}
		})
	}
	wg.Wait()
	// close(limiterChan) <-- we don't close the chan we will use it for further call
	if opts.onJobsDone != nil {
		opts.onJobsDone(jobs)
	}
}
