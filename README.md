# jobExecutor

go module to assist in running jobs in multiple goroutines and print their output

## features:
- Can set the max concurrent jobs with: SetMaxConcurrentJobs, default to runtime. GOMAXPROCS ()
- Can run commands and "runnable" functions (they must return a string and an error)
- Can register handlers for the following events:
	- OnJobsStart: called before any job start
	- OnJobStart: called before each job start
	- OnJobDone: called after each job terminated
	- OnJobsDone: called after all jobs are terminated
- Fluent interface, you can chain methods
- Can add jobs programmatically
- Can display a progress report of ongoing jobs
- Can display output using custom templates

## Usage:

Adding some jobs and executing them

```go
import (
	"errors"
	"fmt"
	"math/rand"
	"time"
	"github.com/software-t-rex/go-jobExecutor"
	"os"
)

func longFunction () (string, error) {
	duration := time.Duration(rand.Intn(5)) * time.Millisecond
	time.Sleep(duration)
	if rand.Intn(10) <= 7 { // random failure
		return fmt.Sprintf("- runnable succeed in %v\n", duration), nil
	}
	return fmt.Sprintf("- runnable Failed in %v\n", duration), errors.New("error while asleep")
}

func longFunction2() (string, error) {
	res, err := runnable()
	if err == nil {
		res = strings.Replace(res, "runnable", "runnable2", -1)
	}
	return res, err
}

func main () {
	// set max concurrent jobs (not required default to GOMAXPROCS)
	jobExecutor.SetMaxConcurrentJobs(8)
	executor := jobExecutor.NewExecutor()
	// add some "runnable" functions
	executor.AddJobFns( longFunction, longFunction2)
	// add a single command
	executor.AddJobCmd("ls", "-l")
	// or multiple command at once
	executor.AddJobsCmd([][]string{
		{"sleep", "5"},
		{"sleep", "2"},
	}...)

	// execute them and get errors if any
	jobErrors := executor.Execute()
	if len(jobErrors) > 0 {
		fmt.Fprintln(os.Stderr, jobErrors)
	}
}

```

Binding some event handlers:
```go
func main () {
	executor := jobExecutor.NewExecutor()

	// add a simple command
	executor.AddJobCmd("sleep", "5")

	// binding some event handlers (can be done anytime before calling Execute)
	// you can call the same method multiple times to bind more than one handler
	// they will be called in order
	executor.
		OnJobsStart(func(jobs jobExecutor.JobList) {
			fmt.Printf("Starting %d jobs\n", len(jobs))
		}).
		OnJobStart(func (jobs jobExecutor.JobList, jobId int) {
			fmt.Printf("Starting jobs %d\n", jobId)
		}).
		OnJobDone(func (jobs jobExecutor.JobList, jobId int) {
			job:=jobs[jobId]
			if job.IsState(jobExecutor.JobStateFailed) {
				fmt.Printf("job %d terminanted with error: %s\n", jobId, job.Err)
			}
		}).
		OnJobsDone(func (jobExecutor.JobList) {
			fmt.Println("Done")
		})

	// add some "runnable" functions and execute
	executor.AddJobFns( longFunction, longFunction2).Execute()
}
```

Display state of running jobs:
```go
func main() {
	jobExecutor.SetMaxConcurrentJobs(3)
	executor := jobExecutor.NewExecutor().WithProgressOutput()
	executor.AddjobCmds( [][]string{
		{"sleep", "10"},
		{"sleep", "9"},
		{"sleep", "8"},
		{"sleep", "7"},
		{"sleep", "6"},
		{"sleep", "5"},
		{"sleep", "4"},
		{"sleep", "3"},
		{"sleep", "2"},
		{"sleep", "1"},
	}...).Execute()
}
```
Other outputs methods:
- WithOrderedOutput: output ordered res and errors at the end
- WithFifoOutput: output res and errors as they arrive
- WithStartOutput: output a line when launching a job
- WithStartSummary: output a summary of jobs to do

All output methods use a go template which you can override by calling the method
```go
jobExecutor.SetTemplateString(myTemplateString)
```
the template string must contains following templates definition:
- startSummary
- jobStatusLine
- jobStatusFull
- doneReport
- startProgressReport
- progressReport
You can look at output.gtpl file for an example