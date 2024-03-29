# jobExecutor

go module to assist in running jobs in multiple goroutines and print their output

## features:
- Can set the max concurrent jobs with: SetMaxConcurrentJobs, default to runtime. GOMAXPROCS ()
- Can run commands and "runnable" functions (they must return a string and an error)
- **Can handle job dependencies** by running them in topological order
- Can register handlers for the following events:
	- OnJobsStart: called before any job start
	- OnJobStart: called before each job start
	- OnJobDone: called after each job terminated
	- OnJobsDone: called after all jobs are terminated
- Fluent interface: you can chain methods call
- Can add jobs programmatically
- Can display a progress report of ongoing jobs
- Can display output using custom templates

## Usage:

### Adding some jobs and executing them

```go
package main

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/software-t-rex/go-jobExecutor"
)

func longFunction() (string, error) {
	duration := time.Duration(rand.Intn(5)) * time.Millisecond
	time.Sleep(duration)
	if rand.Intn(10) <= 7 { // random failure
		return fmt.Sprintf("- runnable succeed in %v\n", duration), nil
	}
	return fmt.Sprintf("- runnable Failed in %v\n", duration), errors.New("error while asleep")
}

func longFunction2() (string, error) {
	res, err := longFunction()
	if err == nil {
		res = strings.Replace(res, "runnable", "runnable2", -1)
	}
	return res, err
}

func main() {
	// set max concurrent jobs (not required default to GOMAXPROCS)
	jobExecutor.SetMaxConcurrentJobs(8)
	executor := jobExecutor.NewExecutor()
	// add some "runnable" functions
	executor.AddJobFns(longFunction, longFunction2)
	// add a single command
	executor.AddJobCmds(exec.Command("ls", "-l"))
	// or multiple command at once
	executor.AddJobCmds(
		exec.Command("sleep", "5"),
		exec.Command("sleep", "2"),
	)
	// there's also AddJob and AddJobs that are not chainable but that returns a job api instead
	myJob := executor.AddJob(exec.Command("sleep", "1"))
	jobs := executor.AddJobs(
		exec.Command("sleep", "2"),
		jobExecutor.NamedJob("MyNamedJob", longFunction) // you can wrap in a NamedJob structure to add job with a name
	)

	// execute them and get errors if any
	jobErrors := executor.Execute()
	if len(jobErrors) > 0 {
		fmt.Fprintln(os.Stderr, jobErrors)
	}
}
```
#### Handling dependencies between jobs
This is based on Directed Acyclic Graph, and using the khan algorithm to topologically sort the jobs.
```go
func main() {
	executor := jobExecutor.NewExecutor()
	jobs := executor.addJobs(
		exec.Command("sleep", "1"),
		exec.Command("exit", "1"),
		exec.Command("ls", "-l"),
	)
	executor.AddDependencies(jobs[0], jobs[1]) // sleep will never run as it depends on a job that always fails
	// execute them respecting dependencies
	jobErrors := executor.DagExecute()
	if len(jobErrors) > 0 {
		fmt.Fprintln(os.Stderr, jobErrors)
	}
}
```

### Binding some event handlers:
```go
func main () {
	executor := jobExecutor.NewExecutor()

	// add a simple command
	executor.AddJobCmds(exec.Command("sleep", "5"))

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

### Display state of running jobs:
```go

func main() {
	jobExecutor.SetMaxConcurrentJobs(5)
	executor := jobExecutor.NewExecutor().WithOngoingStatusOutput()
	// add a command and set its display name in output templates (there's a AddNamedJobFn too)
	executor.AddNamedJobCmd("Wait for 2 seconds", exec.Command("sleep", "2"))

	executor.AddJobCmds(
		exec.Command("sleep", "10"),
		exec.Command("sleep", "9"),
		exec.Command("sleep", "8"),
		exec.Command("sleep", "7"),
		exec.Command("sleep", "6"),
		exec.Command("sleep", "5"),
		exec.Command("sleep", "4"),
		exec.Command("sleep", "3"),
		exec.Command("sleep", "2"),
		exec.Command("sleep", "1"),
	).Execute()
}
```
### Other outputs methods:
- WithProgressBarOutput: Display a progress bar while status are running
- WithOrderedOutput: output ordered res and errors at the end
- WithFifoOutput: output res and errors as they arrive
- WithStartOutput: output a line when launching a job
- WithStartSummary: output a summary of jobs to do
- WithInterleavedOutput: output lines as they arrive prefixed by job name

### Change output formats
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

Alternatively, you can pass a template bound to a specific executor like this:
```go
executor := jobExecutor.NewExecutorWithTemplate(myTemplate)
```

### A note about stdin and stdout
The default behavior of jobExecutor is to run exec.Cmd using the CombinedOutput
method. This allows to print grouped output for jobs as in most of with*Output
methods.
If you have set exec.Cmd.Stdout and/or Stderr, it will then rely on the exec.Cmd.Run
method instead. It won't collect stderr or stdout for you anymore.
Some output methods like the withInterleavedOutput use this internally.
Most of the time this won't impact you as a user of this package, but in case
you're diving in customizing a lot the way you handle the output it may
be important to know how this work.

### Generate a graphviz dot textual representation of the job execution
You can generate a graph representation of the jobs already added to the executor by calling the method GetDot
```go
fmt.println(executor.GetDot())
// output from a test case
digraph G{
	graph [bgcolor="#121212" fontcolor="black" rankdir="RL"]
	node [colorscheme="set312" style="filled,rounded" shape="box"]
	edge [color="#f0f0f0"]
	0 [label="fn 0" color="1"]
	1 [label="fn 1" color="2"]
	2 [label="fn 2" color="3"]
	3 [label="fn 3" color="4"]
	4 [label="cmd 4" color="5"]
	5 [label="cmd 5" color="6"]
	6 [label="cmd 6" color="7"]
	7 [label="cmd 7" color="8"]
	8 [label="cmd 8" color="9"]
	0 -> 1
	0 -> 5
	2 -> 3
	4 -> 7
	6 -> 2
	7 -> 8
	7 -> 0
	{rank=same; 1;3;5;8}
}
```
you can see the result here [https://bit.ly/40wXkwD](https://bit.ly/40wXkwD)

## Contributing
Contributions are welcome, but please make small independent commits when you contribute, it makes the review process a lot easier for me.

## Funding / Sponsorship
If you like my work, and find it useful to you or your company, you can sponsors my work here: [become sponsors to the project](https://github.com/sponsors/malko).