package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

func assert(err error) {
	if err != nil {
		panic(err)
	}
}

type Workflow struct {
	Tag     string
	Run     *github.WorkflowRun
	Job     *github.WorkflowJob
	LogsUrl string
}

func main() {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: "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"},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	fmt.Println("fetching workflows")
	workflows := map[string]*Workflow{}
	total := 1
	for i := 0; i*100 < total; i += 1 {
		fmt.Println("page", i)
		resp, _, err := client.Actions.ListWorkflowRunsByID(context.Background(), "blockscout", "solc-bin", 27157393,
			&github.ListWorkflowRunsOptions{ListOptions: github.ListOptions{PerPage: 100, Page: i}},
		)
		total = *resp.TotalCount
		assert(err)
		if len(resp.WorkflowRuns) == 0 {
			break
		}
		for _, w := range resp.WorkflowRuns {
			workflow := &Workflow{Tag: *w.HeadCommit.Message, Run: w}
			if val, has := workflows[workflow.Tag]; has {
				if val.Run.CreatedAt.Before(workflow.Run.CreatedAt.Time) {
					workflows[workflow.Tag] = workflow
				}
			} else {
				workflows[workflow.Tag] = workflow
			}
		}
	}
	fmt.Println("done fetching workflows:", len(workflows))

	fmt.Println("filtering")
	for tag, run := range workflows {
		if *run.Run.Conclusion == "success" {
			delete(workflows, tag)
		}
	}
	fmt.Println("done filtering:", len(workflows))

	fmt.Println("fetching jobs")
	wait := sync.WaitGroup{}
	for i, w := range workflows {
		wait.Add(1)
		i := i
		w := w
		go func() {
			defer wait.Done()

			jobs, _, err := client.Actions.ListWorkflowJobs(context.Background(), "blockscout", "solc-bin", *w.Run.ID, nil)
			assert(err)
			w.Job = jobs.Jobs[0]
			logs, _, err := client.Actions.GetWorkflowJobLogs(context.Background(), "blockscout", "solc-bin", *w.Job.ID, false)
			assert(err)
			w.LogsUrl = logs.String()

			fmt.Println("fetched", i, w.Tag)

			file, err := os.Create("runs/" + w.Tag + ".json")
			assert(err)
			json.NewEncoder(file).Encode(w)
			file.Close()

			fmt.Println("saved", i, w.Tag)

			file, err = os.Create("runs/" + w.Tag + ".log")
			assert(err)
			resp, err := http.Get(w.LogsUrl)
			assert(err)

			_, err = io.Copy(file, resp.Body)
			assert(err)

			resp.Body.Close()
			file.Close()

			fmt.Println("saved logs", i, w.Tag)
		}()
	}
	wait.Wait()
	fmt.Println("done fetching runs")
}
