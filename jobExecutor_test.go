/*
Copyright © 2023 Jonathan Gotti <jgotti at jgotti dot org>
SPDX-FileType: SOURCE
SPDX-License-Identifier: MIT
SPDX-FileCopyrightText: 2023 Jonathan Gotti <jgotti@jgotti.org>
*/

package jobExecutor

import (
	_ "embed"
	"errors"
	"testing"
)

var TestRunnableSuccessFn = func() (string, error) { return "done", nil }
var TestRunnableFailFn = func() (string, error) { return "", errors.New("test error") }

// check AddJobCmd, AddJobCmds, AddJobFns and Len
func TestJobExecutorJobs(t *testing.T) {
	executor := NewExecutor()
	executor.AddJobCmd("ls", "-l")
	if executor.Len() != 1 {
		t.Fatal("jobExecutor.AddJobCmd doesn't register command properly")
	}
	cmds := [][]string{
		{"cd", "../"},
		{"ls", "-l", "-a"},
	}
	executor.AddJobCmds(cmds...)
	if executor.Len() != 3 {
		t.Fatal("jobExecutor.AddJobCmds doesn't register command properly")
	}
	executor.AddJobFns(TestRunnableSuccessFn, TestRunnableFailFn)
	if executor.Len() != 5 {
		t.Fatal("jobExecutor.AddJobFns doesn't register functions properly")
	}
}

func TestJobExecutorEvents(t *testing.T) {
	var startsCalled int
	var startCalled int
	var doneCalled int
	var donesCalled int
	executor := NewExecutor()
	errs := executor.AddJobFns(TestRunnableSuccessFn, TestRunnableFailFn).
		OnJobsStart(func(jobs JobList) {
			if !(jobs[0].IsState(JobStatePending) && jobs[1].IsState(JobStatePending)) {
				t.Fatal("onJobsStart called with jobs that are not in pending state")
			}
			startsCalled++
		}).
		OnJobStart(func(jobs JobList, jobId int) {
			if !jobs[jobId].IsState(JobStateRunning) {
				t.Fatal("onJobStart called with job that is not in running state")
			}
			startCalled++
		}).
		OnJobDone(func(jobs JobList, jobId int) {
			if !jobs[jobId].IsState(JobStateDone) {
				t.Fatal("onJobDone called with job that is not in done state")
			}
			if jobId == 0 && !jobs[jobId].IsState(JobStateSucceed) {
				t.Fatal("onJobDone job is not properly marked as succeed")
			}
			if jobId == 1 && !jobs[jobId].IsState(JobStateDone) {
				t.Fatal("onJobDone job is not properly marked as failed")
			}
			doneCalled++
		}).
		OnJobsDone(func(jobs JobList) {
			donesCalled++
		}).
		Execute()

	// check events have been called as expected
	if startsCalled != 1 {
		t.Fatalf("OnJobsStart called %d instead of 1", startsCalled)
	} else if startCalled != 2 {
		t.Fatalf("OnJobStart called %d instead of 2", startsCalled)
	} else if doneCalled != 2 {
		t.Fatalf("OnJobDone called %d instead of 2", startsCalled)
	} else if donesCalled != 1 {
		t.Fatalf("OnJobsDone called %d instead of 1", startsCalled)
	}

	// check errors are correctly reported
	if len(errs) != 1 {
		t.Fatalf("Returned %d errors while 1 was expected", len(errs))
	}
}

// test that multiple handler can be attach to single events
func TestJobExecutorEventsHandlers(t *testing.T) {
	var firstCalled int
	var secondCalled int

	executor := NewExecutor().AddJobFns(TestRunnableSuccessFn).
		OnJobsStart(func(JobList) { firstCalled++ }).
		OnJobsStart(func(JobList) { secondCalled++ }).
		OnJobStart(func(JobList, int) { firstCalled++ }).
		OnJobStart(func(JobList, int) { secondCalled++ }).
		OnJobDone(func(JobList, int) { firstCalled++ }).
		OnJobDone(func(JobList, int) { secondCalled++ }).
		OnJobsDone(func(JobList) { firstCalled++ }).
		OnJobsDone(func(JobList) { secondCalled++ })

	executor.Execute()

	if firstCalled != secondCalled || firstCalled != 4 {
		t.Fatalf("expected 4 calls to each handlers received %d and %d call", firstCalled, secondCalled)
	}
}

func TestJobExecutor_Execute(t *testing.T) {
	errs := NewExecutor().
		AddJobFns(TestRunnableSuccessFn, TestRunnableFailFn, TestRunnableFailFn).
		Execute()
	if len(errs) != 2 {
		t.Fatalf("Expected 2 errors received %d", len(errs))
	}
}
