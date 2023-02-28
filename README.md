# jobExecutor

go module to assist in running jobs in multiple goroutines and print their output

## Sample usage:
```go
import "monospace/parralel"

func main () {
	executor = jobExecutor.NewExecutor().WithProgressOutput()
	executor.AddJobFns(
		func() (string, error) {
			// do stuff here
			return "Success", nil
		},
	)
	executor.AddJobCmd("ls", "-l")

	jobErrors := executor.Execute()
}

```

