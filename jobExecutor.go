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
	"math"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"text/template"
)

//go:embed output.gtpl
var dfltTemplateString string
var outputTemplate *template.Template
var ErrCyclicDependencyDetected = fmt.Errorf("cyclic dependencies detected")

type jobEventHandler func(jobs JobList, jobId int)
type jobsEventHandler func(jobs JobList)
type JobExecutor struct {
	jobs     JobList
	opts     *executeOptions
	template *template.Template
}

// ######### template related methods ######### //
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

// ######### internal helpers methods ######### //

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

func getPrintProgress(total int, length int, colorEscSeq string) func(done int32) {
	resetSeq := ""
	if colorEscSeq != "" {
		resetSeq = "\033[0m"
	}
	return func(done int32) {
		// calc percent done
		doneTotal := float64(float32(done) / float32(total) * float32(length) * 8)
		doneStartLength := int(doneTotal / 8)
		rest := math.Mod(doneTotal, 8)
		barStr := strings.Repeat("█", doneStartLength)
		if rest > 0 {
			barStr += string(rune(9616-rest)) + strings.Repeat(" ", length-1-doneStartLength)
		} else {
			barStr += strings.Repeat(" ", length-doneStartLength)
		}
		fmt.Printf(" %s%s%s %d/%d\r", colorEscSeq, barStr, resetSeq, done, total)
	}
}

// return defined template associated with this executor or default template if none
func getExecutorTemplate(e *JobExecutor, name string) *template.Template {
	var tpl *template.Template
	if e.template != nil {
		tpl = e.template.Lookup(name)
	}
	if tpl == nil {
		tpl = outputTemplate.Lookup(name)
	}
	return tpl
}

// ######### public jobExecutor methods ######### //

// Instanciate a new JobExecutor
func NewExecutor() *JobExecutor {
	return NewExecutorWithTemplate(outputTemplate)
}

func NewExecutorWithTemplate(template *template.Template) *JobExecutor {
	executor := &JobExecutor{
		opts:     &executeOptions{},
		template: template,
	}
	return executor
}

// Return the total number of jobs added to the jobExecutor
func (e *JobExecutor) Len() int {
	return len(e.jobs)
}

// ************************** Job Registration **************************//

// Add any kind of supported job to the jobExecutor pool and return a Job
// supported jobs are:
// - an *exec.Cmd
// - a runnableFn (func() (string, error))
// - a NamedJob
// any unsupported job type will panic
// some examples:
//
//	// add an *exec.Cmd
//	cmd := exec.Command("mycommand")
//	job, err := executor.AddJob(cmd)
//	// add runnableFn
//	job, err := executor.AddJob(func() (string, error) {... })
//	// add named *exec.Cmd
//	job, err := executor.AddJob(&jobExecutor.NamedJob{"myjob", cmd))
//	// add named runnableFn
//	job, err := executor.AddJob(&jobExecutor.NamedJob{"myjob", func() (string, error) {... }})
//
// the returned Job can be used to declare dependencies between Jobs
func (e *JobExecutor) AddJob(j interface{}) Job {
	var res Job
	switch typedJob := j.(type) {
	case NamedJob:
		res = e.AddJob(typedJob.Job)
		res.job.displayName = typedJob.Name
		return res
	case *exec.Cmd:
		res = Job{job: &job{id: e.Len(), Cmd: typedJob}}
	case func() (string, error):
		res = Job{job: &job{id: e.Len(), Fn: typedJob}}
	default:
		panic("unsupported job type")
	}
	e.jobs = append(e.jobs, res.job)
	return res
}

// same as AddJob but for multiple jobs at once it will panic on invalid job, and return a slice of added Jobs
func (e *JobExecutor) AddJobs(jobs ...interface{}) []Job {
	res := make([]Job, len(jobs))
	for i, j := range jobs {
		res[i] = e.AddJob(j)
	}
	return res
}

// Add multiple job commands to run.
// This method can be chained.
func (e *JobExecutor) AddJobCmds(cmds ...*exec.Cmd) *JobExecutor {
	for _, cmd := range cmds {
		e.jobs = append(e.jobs, &job{id: e.Len(), Cmd: cmd})
	}
	return e
}

// Add one or more job function to run (func() (string, error)).
// This method can be chained.
func (e *JobExecutor) AddJobFns(fns ...runnableFn) *JobExecutor {
	for _, fn := range fns {
		e.jobs = append(e.jobs, &job{id: e.Len(), Fn: fn})
	}
	return e
}

// Add a job function and set its output display name.
// This method can be chained.
func (e *JobExecutor) AddNamedJobFn(name string, fn runnableFn) *JobExecutor {
	e.jobs = append(e.jobs, &job{id: e.Len(), displayName: name, Fn: fn})
	return e
}

// Add a job command and set its output display name.
// This method can be chained.
func (e *JobExecutor) AddNamedJobCmd(name string, cmd *exec.Cmd) *JobExecutor {
	e.jobs = append(e.jobs, &job{id: e.Len(), displayName: name, Cmd: cmd})
	return e
}

//************************** Events **************************//

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

//************************** Outputs  **************************//

// Output a summary of jobs that will be run
func (e *JobExecutor) WithStartSummary() *JobExecutor {
	e.OnJobsStart(func(jobs JobList) {
		fmt.Print(jobs.execTemplate(getExecutorTemplate(e, "startSummary")))
	})
	return e
}

// Output a line to say a job is starting
func (e *JobExecutor) WithStartOutput() *JobExecutor {
	e.OnJobStart(func(jobs JobList, jobId int) {
		fmt.Print("Starting " + jobs[jobId].execTemplate(getExecutorTemplate(e, "jobStatusLine")))
	})
	return e
}

// Display full jobStatus as they arrive
func (e *JobExecutor) WithFifoOutput() *JobExecutor {
	e.OnJobDone(func(jobs JobList, jobId int) {
		fmt.Print(jobs[jobId].execTemplate(getExecutorTemplate(e, "jobStatusFull")))
	})
	return e
}

// Display doneReport when all jobs are Done
func (e *JobExecutor) WithOrderedOutput() *JobExecutor {
	e.OnJobsDone(func(jobs JobList) {
		fmt.Print(jobs.execTemplate(getExecutorTemplate(e, "doneReport")))
	})
	return e
}

// Print stdout and stderr of command directly to stdout as they arrive
// prefixing the ouput with the job name It overrides cmd.Stdin and cmd.Stdout
// so it won't work well with other With*Output methods that rely on collecting
// them to display them later (typically WithOrderedOutput will have nothing
// to display)
func (e *JobExecutor) WithInterleavedOutput() *JobExecutor {
	e.OnJobsStart(func(jobs JobList) {
		for _, job := range jobs {
			pw := NewPrefixedWriter(os.Stdout, job.Name()+": ")
			if job.Cmd != nil {
				job.Cmd.Stdout = pw
				job.Cmd.Stderr = pw
			} else if job.Fn != nil {
				fn := job.Fn
				job.Fn = func() (string, error) {
					res, err := fn()
					fn = nil
					if res != "" {
						pw.Write([]byte(res))
					}
					if err != nil {
						pw.Write([]byte(err.Error()))
					}
					return res, err
				}
			}
		}
	})
	return e
}

// Display a job status report updated each time a job start or end
// be carefull when dealing with other handler that generate output
// as it will potentially break progress output
func (e *JobExecutor) WithOngoingStatusOutput() *JobExecutor {
	e.OnJobsStart(func(jobs JobList) {
		fmt.Print(jobs.execTemplate(getExecutorTemplate(e, "startProgressReport")))
	})
	printProgress := func(jobs JobList, jobId int) {
		esc := fmt.Sprintf("\033[%dA\033[J", len(jobs)) // clean sequence
		fmt.Print(esc + jobs.execTemplate(getExecutorTemplate(e, "progressReport")))
	}
	e.OnJobDone(printProgress)
	e.OnJobStart(printProgress)
	return e
}

// - length is the number of characters used to print the progress bar
// - keepOnDone determines if the progress bar should be kept on the screen when done or not
// - colorEscSeq is an ANSII terminal escape sequence ie: "\033[32m"
func (e *JobExecutor) WithProgressBarOutput(length int, keepOnDone bool, colorEscSeq string) *JobExecutor {
	var doneCount atomic.Int32
	var printProgress func(done int32)
	e.OnJobsStart(func(jobs JobList) {
		printProgress = getPrintProgress(e.Len(), length, colorEscSeq)
	})
	e.OnJobDone(func(jobs JobList, jobId int) {
		doneCount.Add(1)
		printProgress(doneCount.Load())
	})
	e.OnJobStart(func(jobs JobList, jobId int) { printProgress(doneCount.Load()) })
	e.OnJobsDone(func(jobs JobList) {
		if keepOnDone {
			fmt.Print("\n") // go to next line
		} else {
			fmt.Print("\033[2K") // clear line
		}
	})
	return e
}

//************************** Run jobs **************************//

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

// Register "from" job as dependent on "to" job
func (e *JobExecutor) AddJobDependency(from Job, to Job) *JobExecutor {
	from.job.DependsOn = append(from.job.DependsOn, to.job)
	return e //, nil
}

// Check that the jobs registered in the executor don't make a cyclic dependency
// (use Kahn's topological sort algorithm)
func (e *JobExecutor) IsAcyclic() bool {
	length := e.Len()
	if length < 1 {
		return true
	}
	// init an adjacencyList to store edges between jobs
	adjacencyList := make(map[int][]int, length)
	// Count dependent on each job
	dependentCount := make(map[int]int, length)
	for _, job := range e.jobs {
		for _, to := range job.DependsOn {
			adjacencyList[job.id] = append(adjacencyList[job.id], to.id)
			dependentCount[to.id]++
		}
	}

	// Find all start jobs
	var queue []int
	for _, job := range e.jobs {
		if dependentCount[job.id] == 0 {
			queue = append(queue, job.id)
		}
	}

	index := 0
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		index++
		for _, to := range e.jobs[at].DependsOn {
			// for _, to := range adjacencyList[at] {
			dependentCount[to.id]--
			if dependentCount[to.id] == 0 {
				queue = append(queue, to.id)
			}
		}
	}

	return index == length
}

func (e *JobExecutor) DagExecute() JobsError {
	var errs = make([]error, e.Len())
	var res = make(JobsError, e.Len())
	e.OnJobDone(func(jobs JobList, jobId int) {
		err := jobs[jobId].Err
		if err != nil {
			errs[jobId] = err
		}
	})
	if !e.IsAcyclic() {
		for jobId := range e.jobs {
			res[jobId] = ErrCyclicDependencyDetected
		}
		return res
	}
	// no cyclic dependency detected call execute
	dagExecute(e.jobs, *e.opts)
	for jobId, err := range errs {
		if err != nil {
			res[jobId] = err
		}
	}
	return res
}

// return a graphviz dot representation of the execution graph you can render it
// using graphviz or pasting output to https://dreampuf.github.io/GraphvizOnline/
func (e *JobExecutor) GetDot() string {
	out := []string{`digraph G{
	graph [bgcolor="#121212" fontcolor="black" rankdir="RL"]
	node [colorscheme="set312" style="filled,rounded" shape="box"]
	edge [color="#f0f0f0"]`}
	for _, j := range e.jobs {
		out = append(out, fmt.Sprintf("\t%d [label=\"%s\" color=\"%d\"]", j.id, j.Name(), j.id%12+1))
	}
	for _, j := range e.jobs {
		for _, dep := range j.DependsOn {
			out = append(out, fmt.Sprintf("\t%d -> %d", j.id, dep.id))
		}
	}
	// finally group all nodes without dependencies
	noDepNodes := []string{}
	for _, j := range e.jobs {
		if len(j.DependsOn) == 0 {
			noDepNodes = append(noDepNodes, fmt.Sprintf("%d", j.id))
		}
	}
	if len(noDepNodes) > 1 {
		out = append(out, fmt.Sprintf("\t{rank=same; %s}", strings.Join(noDepNodes, ";")))
	}

	return strings.Join(out, "\n") + "\n}"
}
