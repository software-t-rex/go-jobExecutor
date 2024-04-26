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
	"os/exec"
	"runtime"
	"sync"
	"testing"
)

var TestRunnableSuccessFn = func() (string, error) { return "done", nil }
var TestRunnableFailFn = func() (string, error) { return "", errors.New("test error") }

// check AddJobCmd, AddJobCmds, AddJobFns and Len
func TestJobExecutorJobs(t *testing.T) {
	executor := NewExecutor()
	executor.AddJobCmds(exec.Command("ls", "-l"))
	if executor.Len() != 1 {
		t.Fatal("jobExecutor.AddJobCmd doesn't register command properly")
	}
	cmds := []*exec.Cmd{
		exec.Command("cd", "../"),
		exec.Command("ls", "-l", "-a"),
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

func TestJobExecutor_AddJob(t *testing.T) {
	asserts := map[string]func(*testing.T, *JobExecutor, Job) bool{}
	asserts["added"] = func(t *testing.T, e *JobExecutor, j Job) bool {
		return e.Len() == 1
	}
	asserts["IsFnJob"] = func(t *testing.T, e *JobExecutor, j Job) bool { return j.IsFnJob() }
	asserts["IsCmdJob"] = func(t *testing.T, e *JobExecutor, j Job) bool { return j.IsCmdJob() }
	asserts["hasTestName"] = func(t *testing.T, e *JobExecutor, j Job) bool { return j.Name() == "test" }

	tests := []struct {
		name      string
		job       interface{}
		asserts   []string
		wantPanic bool
	}{
		// TODO: Add test cases.
		{"Adding arbitrary stuff should return an error", func() {}, nil, true},
		{"Adding a runnableFn shoud add it to the executor", func() (string, error) { return "", nil }, []string{"added", "IsFnJob"}, false},
		{"Adding an execCmd shoud add it to the executor", exec.Command("exit"), []string{"added", "IsCmdJob"}, false},
		{"Adding a runnableFn shoud add it to the executor", NamedJob{"test", func() (string, error) { return "", nil }}, []string{"added", "IsFnJob", "hasTestName"}, false},
		{"Adding an execCmd shoud add it to the executor", NamedJob{"test", exec.Command("exit")}, []string{"added", "IsCmdJob", "hasTestName"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.wantPanic && r == nil {
					t.Errorf("The code did not panic")
				} else if !tt.wantPanic && r != nil {
					t.Errorf("unnatended panic")
				}
			}()
			executor := &JobExecutor{opts: &executeOptions{}}
			gotRes := executor.AddJob(tt.job)
			if len(tt.asserts) > 0 {
				for _, assert := range tt.asserts {
					if !asserts[assert](t, executor, gotRes) {
						t.Errorf("JobExecutor.AddJob() failed assertion %s", assert)
					}
				}
			}
		})
	}
}

func TestJobExecutorEvents(t *testing.T) {
	var startsCalled int
	var startCalled int
	var doneCalled int
	var donesCalled int
	mutex := &sync.Mutex{}
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
			mutex.Lock()
			startCalled++
			mutex.Unlock()
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
			mutex.Lock()
			doneCalled++
			mutex.Unlock()
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

func TestJobExecutor_IsAcyclic(t *testing.T) {
	// testing no cycle
	e1 := NewExecutor()
	e1Jobs := []Job{
		e1.AddJob(TestRunnableSuccessFn),
		e1.AddJob(TestRunnableSuccessFn),
		e1.AddJob(TestRunnableSuccessFn),
	}
	if !e1.IsAcyclic() {
		t.Fatalf("Should return true when there is no dependecy defined")
	}
	e1.AddJobDependency(e1Jobs[0], e1Jobs[1])
	e1.AddJobDependency(e1Jobs[2], e1Jobs[0])
	if !e1.IsAcyclic() {
		t.Fatalf("Should return true when there is no cycle")
	}
	e1.AddJobDependency(e1Jobs[1], e1Jobs[2])
	e1.AddJobDependency(e1Jobs[0], e1Jobs[2])
	if e1.IsAcyclic() {
		t.Fatalf("Should return false when there is a cycle")
	}
	e2 := NewExecutor()
	e2Jobs := e2.AddJobs(
		TestRunnableSuccessFn,     // 0 -> 1
		exec.Command("exit", "0"), // 1 -> 2, 3
		exec.Command("exit", "1"), // - 2 -> 3
		TestRunnableFailFn,        // - 3 -> 0
	)
	e2.AddJobDependency(e2Jobs[0], e2Jobs[1])
	e2.AddJobDependency(e2Jobs[1], e2Jobs[2])
	e2.AddJobDependency(e2Jobs[1], e2Jobs[3])
	e2.AddJobDependency(e2Jobs[2], e2Jobs[3])
	if !e2.IsAcyclic() {
		t.Fatalf("Should return true when there is no cycle")
	}
	e2.AddJobDependency(e2Jobs[3], e2Jobs[0])
	if e2.IsAcyclic() {
		t.Fatalf("Should return true when there is no cycle")
	}
}

func TestJobExecutor_DagExecute(t *testing.T) {
	e := NewExecutor()
	var jobs []Job
	if runtime.GOOS == "windows" {
		jobs = []Job{
			e.AddJob(NamedJob{"fn 0", TestRunnableSuccessFn}),                               // 0 ->  1, 5 / <- 7
			e.AddJob(NamedJob{"fn 1", TestRunnableSuccessFn}),                               // 1 <- 0
			e.AddJob(NamedJob{"fn 2", TestRunnableSuccessFn}),                               // 2 -> 3 / <- 6
			e.AddJob(NamedJob{"fn 3", TestRunnableFailFn}),                                  // 3 <- 2
			e.AddJob(NamedJob{"cmd 4", exec.Command("cmd", "/C", "start", "timeout", "1")}), // 4 -> 7
			e.AddJob(NamedJob{"cmd 5", exec.Command("cmd", "/C", "start", "timeout", "1")}), // 5 <- 0
			e.AddJob(NamedJob{"cmd 6", exec.Command("cmd", "/C", "start", "timeout", "1")}), // 6 -> 2
			e.AddJob(NamedJob{"cmd 7", exec.Command("cmd", "/C", "start", "timeout", "1")}), // 7 -> 8, 0 / <- 4
			e.AddJob(NamedJob{"cmd 8", exec.Command("bash", "-c", "exit 1")}),               // 8 <- 7 will exit if command not found that's ok for the test
		}
	} else {
		jobs = []Job{
			e.AddJob(NamedJob{"fn 0", TestRunnableSuccessFn}),                  // 0 ->  1, 5 / <- 7
			e.AddJob(NamedJob{"fn 1", TestRunnableSuccessFn}),                  // 1 <- 0
			e.AddJob(NamedJob{"fn 2", TestRunnableSuccessFn}),                  // 2 -> 3 / <- 6
			e.AddJob(NamedJob{"fn 3", TestRunnableFailFn}),                     // 3 <- 2
			e.AddJob(NamedJob{"cmd 4", exec.Command("bash", "-c", "sleep 1")}), // 4 -> 7
			e.AddJob(NamedJob{"cmd 5", exec.Command("bash", "-c", "sleep 1")}), // 5 <- 0
			e.AddJob(NamedJob{"cmd 6", exec.Command("bash", "-c", "sleep 1")}), // 6 -> 2
			e.AddJob(NamedJob{"cmd 7", exec.Command("bash", "-c", "sleep 1")}), // 7 -> 8, 0 / <- 4
			e.AddJob(NamedJob{"cmd 8", exec.Command("bash", "-c", "exit 1")}),  // 8 <- 7
		}
	}
	// define dependencies
	e.
		AddJobDependency(jobs[0], jobs[1]).
		AddJobDependency(jobs[0], jobs[5]).
		AddJobDependency(jobs[2], jobs[3]).
		AddJobDependency(jobs[4], jobs[7]).
		AddJobDependency(jobs[6], jobs[2]).
		AddJobDependency(jobs[7], jobs[8]).
		AddJobDependency(jobs[7], jobs[0]).
		// WithOngoingStatusOutput().
		WithFifoOutput()

	errs := e.DagExecute()
	// job 3, 8 fail naturraly
	// job 2,4,7 should failed as their dependencies failed
	type stateTest struct {
		name string
		want int
	}
	stateTests := []stateTest{
		{"job with succeeding dependency should succeed", JobStateSucceed}, //0
		{"job with no dependency should succeed", JobStateSucceed},         //1
		{"job with failing dependency should fail", JobStateFailed},        //2 depends on 3 which fails
		{"job failing should fail", JobStateFailed},                        //3 born to fail
		{"job with failing dependency should fail", JobStateFailed},        //4 depends on 7 which fails
		{"job with no dependency should succeed", JobStateSucceed},         //5
		{"job with failing dependency should fail", JobStateFailed},        //6 // depends on 2 which depends on 3 which fail
		{"job with failing dependency should fail", JobStateFailed},        //7 depends on 8 which fail
		{"job failing should fail", JobStateFailed},                        //8 born to fail
	}

	for i, st := range stateTests {
		t.Run(st.name, func(t *testing.T) {
			if !jobs[i].IsState(st.want) {
				t.Errorf("Job %d(%s) should match state %d, has state: %d", i, jobs[i].Name(), st.want, jobs[i].job.status)
			}
		})
	}
	if len(errs) != 6 {
		t.Fatalf("Expected 6 errors received %d", len(errs))
	}

	// testing error on cyclic dep
	e2 := NewExecutor()
	jobs2 := []Job{
		e2.AddJob(NamedJob{"fn 0", TestRunnableSuccessFn}), // -> 1
		e2.AddJob(NamedJob{"fn 1", TestRunnableSuccessFn}), // -> 2
		e2.AddJob(NamedJob{"fn 2", TestRunnableSuccessFn}), // -> 0
		e2.AddJob(NamedJob{"fn 3", TestRunnableSuccessFn}),
	}
	e2.AddJobDependency(jobs2[0], jobs2[1])
	e2.AddJobDependency(jobs2[1], jobs2[2])
	e2.AddJobDependency(jobs2[2], jobs2[0])
	errs2 := e2.DagExecute()
	if len(errs2) != len(jobs2) {
		t.Fatalf("Expected all jobs on error when cyclic dependency detected got: %d, expected: %d", len(errs2), len(jobs2))
	}
	for _, err := range errs2 {
		if !errors.Is(err, ErrCyclicDependencyDetected) {
			t.Fatalf("Expected cyclic dependency error, got %v", err)
		}
	}
}
