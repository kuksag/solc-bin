package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-git/v5"
)

func shaFromVersion(ver string) string {
	return ver[len(ver)-8:]
}

func main() {
	shaFile := "versions_sha.txt"
	var versionFile string
	flag.StringVar(&versionFile, "version-file", "", "")
	flag.Parse()
	if versionFile == "" {
		panic("empty version file")
	}
	templateFile := "build.yml"

	shas, err := os.Open(shaFile)
	if err != nil {
		panic(err)
	}

	versions, err := os.Open(versionFile)
	if err != nil {
		panic(err)
	}

	tmpl, err := template.ParseFiles(templateFile)
	if err != nil {
		panic(err)
	}

	shaMap := map[string]string{}
	scanner := bufio.NewScanner(shas)
	for scanner.Scan() {
		line := scanner.Text()
		shaMap[line[:8]] = line
	}

	versionsArr := []string{}
	versionSha := map[string]string{}
	versionName := map[string]string{}
	versionSemVer := map[string]*semver.Version{}

	scanner = bufio.NewScanner(versions)
	for scanner.Scan() {
		line := scanner.Text()
		versionSha[shaFromVersion(line)] = line
		split := strings.Split(line[5:], "+")
		ver := split[0]
		versionName[ver] = line
		sv := strings.Split(ver, "-")
		versionSemVer[ver], err = semver.NewVersion(sv[0][1:])
		versionsArr = append(versionsArr, ver)
		if err != nil {
			panic(err)
		}
	}

	sort.Slice(versionsArr, func(i, j int) bool {
		return versionSemVer[versionsArr[i]].LessThan(versionSemVer[versionsArr[j]])
	})

	r, err := git.PlainOpen("./")
	if err != nil {
		panic(err)
	}
	w, err := r.Worktree()
	if err != nil {
		panic(err)
	}

	// constraint, err := semver.NewConstraint(">= 0.5.0")
	// if err != nil {
	// 	panic(err)
	// }

	gcc8Constraint, err := semver.NewConstraint("< 0.5.10")
	if err != nil {
		panic(err)
	}
	gcc7Constraint, err := semver.NewConstraint("< 0.5.1")
	if err != nil {
		panic(err)
	}

	buildFileName := ".github/workflows/build.yml"

	for _, ver := range versionsArr {
		// if !constraint.Check(versionSemVer[ver]) {
		// 	continue
		// }
		// if !strings.Contains(ver, "nightly") {
		// 	continue
		// }

		fmt.Println("pushing", ver)

		name := versionName[ver]
		sha := shaMap[shaFromVersion(name)]

		type Data struct {
			TagName    string
			CommitHash string
			Gcc8       bool
			Gcc7       bool
			Release    bool
		}

		data := Data{
			TagName:    name,
			CommitHash: sha,
			Gcc8:       gcc8Constraint.Check(versionSemVer[ver]),
			Gcc7:       gcc7Constraint.Check(versionSemVer[ver]),
			Release:    !strings.Contains(ver, "nightly"),
		}
		build, err := os.Create(buildFileName)
		if err != nil {
			panic(err)
		}
		if err := tmpl.Execute(build, data); err != nil {
			panic(err)
		}
		if _, err = w.Add(buildFileName); err != nil {
			fmt.Println(err)
		}
		if _, err = w.Commit(name, &git.CommitOptions{}); err != nil {
			fmt.Println(err)
		}
		if err = r.Push(&git.PushOptions{}); err != nil {
			fmt.Println(err)
		}
	}
}
