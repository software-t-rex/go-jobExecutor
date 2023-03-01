/*
Copyright © 2023 Jonathan Gotti <jgotti at jgotti dot org>
SPDX-FileType: SOURCE
SPDX-License-Identifier: MIT
SPDX-FileCopyrightText: 2023 Jonathan Gotti <jgotti@jgotti.org>
*/
package jobExecutor

import (
	"os/exec"
	"testing"
)

func Test_job_run(t *testing.T) {
	var doneCalled bool
	j := job{Fn: func() (string, error) { return "done", nil }}

	if !j.IsState(JobStatePending) {
		t.Fatalf("Job not marked as Pending")
	}
	j.run(func() { doneCalled = true })
	if !doneCalled {
		t.Fatalf("run did not call done")
	}
	if !j.IsState(JobStateDone) {
		t.Fatalf("Job not marked as done")
	}
}

func Test_job_Name(t *testing.T) {
	j := job{Cmd: exec.Command("ls", "-l")}
	got := j.Name()
	want := "ls -l"
	if got != want {
		t.Fatalf("job.Name() = %v, want %v", got, want)
	}
}

func Test_job_IsState(t *testing.T) {
	type args struct {
		jobState int
	}
	tests := []struct {
		name string
		j    *job
		args args
		want bool
	}{
		{"IsState(JobStatePending)", &job{status: JobStatePending}, args{jobState: JobStatePending}, true},
		{"IsState(JobStatePending)", &job{status: JobStateRunning}, args{jobState: JobStatePending}, false},
		{"IsState(JobStatePending)", &job{status: JobStateDone}, args{jobState: JobStatePending}, false},
		{"IsState(JobStatePending)", &job{status: JobStateDone | JobStateSucceed}, args{jobState: JobStatePending}, false},
		{"IsState(JobStatePending)", &job{status: JobStateDone | JobStateFailed}, args{jobState: JobStatePending}, false},

		{"IsState(JobStateRunning)", &job{status: JobStatePending}, args{jobState: JobStateRunning}, false},
		{"IsState(JobStateRunning)", &job{status: JobStateRunning}, args{jobState: JobStateRunning}, true},
		{"IsState(JobStateRunning)", &job{status: JobStateDone}, args{jobState: JobStateRunning}, false},
		{"IsState(JobStateRunning)", &job{status: JobStateDone | JobStateSucceed}, args{jobState: JobStateRunning}, false},
		{"IsState(JobStateRunning)", &job{status: JobStateDone | JobStateFailed}, args{jobState: JobStateRunning}, false},

		{"IsState(JobStateDone)", &job{status: JobStatePending}, args{jobState: JobStateDone}, false},
		{"IsState(JobStateDone)", &job{status: JobStateRunning}, args{jobState: JobStateDone}, false},
		{"IsState(JobStateDone)", &job{status: JobStateDone}, args{jobState: JobStateDone}, true},
		{"IsState(JobStateDone)", &job{status: JobStateDone | JobStateSucceed}, args{jobState: JobStateDone}, true},
		{"IsState(JobStateDone)", &job{status: JobStateDone | JobStateFailed}, args{jobState: JobStateDone}, true},

		{"IsState(JobStateSucceed)", &job{status: JobStatePending}, args{jobState: JobStateSucceed}, false},
		{"IsState(JobStateSucceed)", &job{status: JobStateRunning}, args{jobState: JobStateSucceed}, false},
		{"IsState(JobStateSucceed)", &job{status: JobStateDone}, args{jobState: JobStateSucceed}, false},
		{"IsState(JobStateSucceed)", &job{status: JobStateDone | JobStateSucceed}, args{jobState: JobStateSucceed}, true},
		{"IsState(JobStateSucceed)", &job{status: JobStateDone | JobStateFailed}, args{jobState: JobStateSucceed}, false},

		{"IsState(JobStateFailed)", &job{status: JobStatePending}, args{jobState: JobStateFailed}, false},
		{"IsState(JobStateFailed)", &job{status: JobStateRunning}, args{jobState: JobStateFailed}, false},
		{"IsState(JobStateFailed)", &job{status: JobStateDone}, args{jobState: JobStateFailed}, false},
		{"IsState(JobStateFailed)", &job{status: JobStateDone | JobStateSucceed}, args{jobState: JobStateFailed}, false},
		{"IsState(JobStateFailed)", &job{status: JobStateDone | JobStateFailed}, args{jobState: JobStateFailed}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.j.IsState(tt.args.jobState); got != tt.want {
				t.Errorf("job.IsState(%v) = %v, want %v", tt.args.jobState, got, tt.want)
			}
		})
	}
}

// TestJobTemplates Check common templates are defined
func Test_job_Templates(t *testing.T) {
	jobTemplates := []string{
		"jobStatusLine",
		"jobStatusFull",
	}
	jobListTemplates := []string{
		"doneReport",
		"startSummary",
		"startProgressReport",
		"progressReport",
		// "undefinedss",
	}
	testjob := &job{Fn: func() (string, error) {
		return "done", nil
	}}
	testJobList := JobList{testjob}

	for _, tplName := range jobTemplates {
		out := testjob.execTemplate(tplName)
		if out == "" || out == "undefined" {
			t.Fatalf(`Empty job template %s`, tplName)
		}
	}
	for _, tplName := range jobListTemplates {
		out := testJobList.execTemplate(tplName)
		if out == "" || out == "undefined" {
			t.Fatalf(`Empty JobList template %s`, tplName)
		}
	}
}
