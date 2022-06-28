package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/google/go-github/v45/github"
)

type Workflow struct {
	Tag      string
	Run      *github.WorkflowRun
	Job      *github.WorkflowJob
	LogsUrl  string
	JsonFile string
	LogsFile string
	Error    string
}

func assert(err error) {
	if err != nil {
		panic(err)
	}
}

func removeExtension(s string) string {
	return s[:len(s)-len(filepath.Ext(s))]
}

func runGrep(s string, file string) string {
	cmd := exec.Command("grep", s, file)
	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	cmd.Run()
	return buffer.String()
}

func main() {
	files, err := filepath.Glob("runs/*.json")
	assert(err)

	workflows := []*Workflow{}
	fmt.Println("started reading:", len(files))
	for _, filePath := range files {
		fmt.Println("reading", filePath)
		file, err := os.Open(filePath)
		assert(err)

		var workflow Workflow
		err = json.NewDecoder(file).Decode(&workflow)
		assert(err)

		workflow.JsonFile = filePath
		workflow.LogsFile = removeExtension(filePath) + ".log"
		workflows = append(workflows, &workflow)
	}
	fmt.Println("done reading:", len(workflows))

	sort.Slice(workflows, func(i, j int) bool {
		return workflows[i].Tag < workflows[j].Tag
	})
	filtered := map[string][]*Workflow{}

	fmt.Println("\n\nstarted filtering")
	for _, w := range workflows {
		fmt.Println("filtering", w.LogsFile)
		if len(runGrep("Can't use 'tar -xzf' extract archive file", w.LogsFile)) > 0 {
			w.Error = "checkout"
		} else if len(runGrep("Response status code does not indicate success: 503 (Service Unavailable)", w.LogsFile)) > 0 {
			w.Error = "checkout"
		} else if len(runGrep("error: downloading 'https://github.com/ericniebler/range-v3/archive/0.11.0.tar.gz' failed", w.LogsFile)) > 0 {
			w.Error = "checkout"
		} else if len(runGrep("redundant move in return statement", w.LogsFile)) > 0 {
			w.Error = "move"
		} else if len(runGrep("SMTChecker tests require.*for all tests to pass", w.LogsFile)) > 0 {
			w.Error = "z3"
		}

		if w.Error == "" {
			w.Error = "unfiltered"
		}
		filtered[w.Error] = append(filtered[w.Error], w)
	}
	fmt.Println("done filtering")

	fmt.Println("\n\nstarted saving")
	for name, workflows := range filtered {
		fmt.Println("saving", name)
		file, err := os.Create("filtered/" + name + ".txt")
		assert(err)
		for _, w := range workflows {
			fmt.Fprintln(file, w.Tag)
		}
	}
	fmt.Println("done saving")

}
