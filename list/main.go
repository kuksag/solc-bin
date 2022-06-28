package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

func unwrap(err error) {
	if err != nil {
		panic(err)
	}
}

//solc-v0.8.9+commit.e5eed63a
//solc-v0.8.8-nightly.2021.9.9+commit.dea1b9ec
func newSemVer(s string) (string, string, string) {
	var tmp []string

	s = strings.Split(s, "-v")[1]

	tmp = strings.Split(s, "+")
	commit := tmp[1]
	s = tmp[0]

	tmp = strings.Split(s, "-nightly.")
	ver := tmp[0]
	var date string
	if len(tmp) > 1 {
		date = tmp[1]
	}
	return ver, date, commit
}

func newSort(s string) (*semver.Version, time.Time, bool) {
	verStr, dateStr, _ := newSemVer(s)
	var date time.Time
	if len(dateStr) > 0 {
		var err error
		date, err = time.Parse("2006.1.2", dateStr)
		unwrap(err)
	}
	ver, err := semver.NewVersion(verStr)
	unwrap(err)
	return ver, date, strings.Contains(s, "nightly")
}

type ListElem struct {
	Path        string `json:"path"`
	Version     string `json:"version"`
	Prerelease  string `json:"prerelease,omitempty"`
	Build       string `json:"build"`
	LongVersion string `json:"longVersion"`
	SHA256      string `json:"sha256"`
	MD5         string `json:"md5"`
}

type ListJson struct {
	Builds []*ListElem `json:"builds"`
}

func getFileContent(url string) string {
	resp, err := http.Get(url)
	unwrap(err)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		panic("wrong status")
	}
	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, resp.Body)
	unwrap(err)
	return strings.ReplaceAll(buf.String(), "\n", "")
}

func main() {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: "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"},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	releases := []*github.RepositoryRelease{}
	for page := 0; ; page++ {
		page, _, err := client.Repositories.ListReleases(ctx, "blockscout", "solc-bin", &github.ListOptions{Page: page, PerPage: 100})
		unwrap(err)
		if len(page) == 0 {
			break
		}
		releases = append(releases, page...)
	}

	sort.Slice(releases, func(i, j int) bool {
		iVer, iDate, iNightly := newSort(*releases[i].Name)
		jVer, jDate, jNightly := newSort(*releases[j].Name)
		return iVer.LessThan(jVer) ||
			iVer.Equal(jVer) && (iNightly && !jNightly ||
				iNightly && jNightly && iDate.Before(jDate))
	})

	list := ListJson{
		Builds: make([]*ListElem, 0, len(releases)),
	}

	wait := sync.WaitGroup{}
	for _, release := range releases {
		fmt.Println("processing", *release.Name)
		var elem ListElem
		ver, date, commit := newSemVer(*release.Name)
		elem.Version = ver
		if len(date) > 0 {
			elem.Prerelease = "nightly." + date
			elem.LongVersion = elem.Version + "-" + elem.Prerelease + "+" + commit
		} else {
			elem.LongVersion = elem.Version + "+" + commit
		}
		elem.Build = commit

		files := map[string]*github.ReleaseAsset{}
		for _, asset := range release.Assets {
			files[*asset.Name] = asset
		}

		wait.Add(1)
		go func() {
			defer wait.Done()
			elem.MD5 = getFileContent(*files["md5.hash"].BrowserDownloadURL)

			fmt.Println("downloaded md5", elem.LongVersion)
		}()

		wait.Add(1)
		go func() {
			defer wait.Done()
			elem.SHA256 = getFileContent(*files["sha256.hash"].BrowserDownloadURL)

			fmt.Println("downloaded sha256", elem.LongVersion)
		}()
		elem.Path = *files["solc"].BrowserDownloadURL

		list.Builds = append(list.Builds, &elem)
	}
	wait.Wait()
	output, err := os.Create("list.json")
	unwrap(err)
	json.NewEncoder(output).Encode(list)
}
