package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	git "github.com/go-git/go-git/v5"
	helper "github.com/go-git/go-git/v5/_examples"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type ScmWorkflowDetails struct {
	ScProvider      string
	GitOrgProject   string
	GitRepository   string
	OAuthToken      string
	CloneURL        string
	Branch          string
	CommitMsg       string
	CommitId        string
	CommitUrl       string
	Email     	    string
	Workflow    	Workflow
}
// Created with https://yaml.to-go.online/
// https://raw.githubusercontent.com/agnops/examples/master/.agnops/workflow-with-everything.yaml
type WorkflowYaml struct {
	Workflow struct {
		AutoTrigger  bool `yaml:"autoTrigger"`
		GlobalAddOns struct {
			RAMDisk        string   `yaml:"ramDisk"`
			RepoName       string   `yaml:"repoName"`
			DockerFilePath string   `yaml:"dockerFilePath"`
			DockerCloudOps []string `yaml:"dockerCloudOps"`
		} `yaml:"globalAddOns"`
		CloudFilters  []string `yaml:"cloudFilters"`
		BranchFilters []string `yaml:"branchFilters"`
		TrackedFiles  []string `yaml:"trackedFiles"`
		Containers    []struct {
			Container interface{} `yaml:"container"`
			Name      string      `yaml:"name"`
			Image     string      `yaml:"image"`
			Command   string      `yaml:"command"`
			AddOns    struct {
				IsDocker bool `yaml:"isDocker"`
			} `yaml:"addOns,omitempty"`
			Kubernetes struct {
				EnvFrom []struct {
					SecretRef struct {
						Name string `yaml:"name"`
					} `yaml:"secretRef"`
				} `yaml:"envFrom"`
				Resources struct {
					Limits struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					} `yaml:"limits"`
					Requests struct {
						CPU    string `yaml:"cpu"`
						Memory string `yaml:"memory"`
					} `yaml:"requests"`
				} `yaml:"resources"`
			} `yaml:"kubernetes,omitempty"`
		} `yaml:"containers"`
	} `yaml:"workflow"`
}

type Workflow struct {
	FileName		string
	WorkflowYaml	WorkflowYaml
}

func checkModifiedFiles(modifiedFiles[] string, trackedFiles[] string) bool {

	if len(trackedFiles) > 0 {
		for _, md := range modifiedFiles {
			for _, tf := range trackedFiles {
				if strings.Contains(md, tf) {
					return true
				}
			}
		}
		return false
	} else {
		return true
	}
}

func checkBranchFilters(branchFilters[] string, branch string) bool {

	if len(branchFilters) > 0 {
		for _, bf := range branchFilters {
			cmp := "(?m)" + strings.ReplaceAll(bf, "/", "")
			var re = regexp.MustCompile(cmp)
			for range re.FindAllString(branch, -1) {
				return true
			}
		}
		return false
	} else {
		return true
	}
}

func checkCloudFilters(cloudFilters[] string) bool {
	return checkBranchFilters(cloudFilters, cloudName)
}

func checkGitWorkflowExistInRepo(clone_url string, git_org_project string, git_repository string, commit string, token string, token_user string, modifiedFiles []string, branch string) ([]Workflow, error) {

	curWd, _ := os.Getwd()
	repoClonePath := path.Join(curWd, "repos", git_org_project, git_repository, commit)
	cicdJobYamlPath := path.Join(repoClonePath + "/.agnops")

	// Clone the given repository to the given directory
	helper.Info("git clone %s %s", clone_url, repoClonePath)

	if _, dirErr := os.Stat(repoClonePath); os.IsNotExist(dirErr) {
		r, err := git.PlainClone(repoClonePath, false, &git.CloneOptions{
			Auth: &http.BasicAuth{
				Username: token_user,
				Password: token,
			},
			URL:      clone_url,
			Progress: os.Stdout,
		})
		helper.CheckIfError(err)
		// ... retrieving the commit being pointed by HEAD
		helper.Info("git show-ref --head HEAD")
		ref, err := r.Head()
		helper.CheckIfError(err)
		fmt.Println(ref.Hash())
		w, err := r.Worktree()
		helper.CheckIfError(err)
		// ... checking out to commit
		helper.Info("git checkout %s", commit)
		err = w.Checkout(&git.CheckoutOptions{
			Hash: plumbing.NewHash(commit),
		})
		helper.CheckIfError(err)

		// ... retrieving the commit being pointed by HEAD, it shows that the
		// repository is pointing to the giving commit in detached mode
		helper.Info("git show-ref --head HEAD")
		ref, err = r.Head()
		helper.CheckIfError(err)
		fmt.Println(ref.Hash())
	}

	_, err := os.Stat(cicdJobYamlPath)

	var workflows []Workflow

	if err == nil {
		var files []string

		filepath.Walk(cicdJobYamlPath, func(path string, f os.FileInfo, _ error) error {
			if !f.IsDir() {
				r, err := regexp.MatchString(".yaml", f.Name())
				if err == nil && r {
					files = append(files, f.Name())
				}
			}
			return nil
		})

		if len(files) > 0 {
			var content []byte
			for _, yamlFile := range files {
				content, err = ioutil.ReadFile(path.Join(cicdJobYamlPath, yamlFile))
				workflowYaml := WorkflowYaml{}
				err := yaml.Unmarshal(content, &workflowYaml)
				if err == nil {
					if checkModifiedFiles(modifiedFiles, workflowYaml.Workflow.TrackedFiles) && checkBranchFilters(workflowYaml.Workflow.BranchFilters, branch) && checkCloudFilters(workflowYaml.Workflow.CloudFilters) {
						workflows = append(workflows, Workflow{FileName: yamlFile, WorkflowYaml: workflowYaml})
					}
				} else {
					failOnError(err, "Failed to unmarshal the workflow file: " + yamlFile)
					workflows = append(workflows, Workflow{FileName: yamlFile, WorkflowYaml: WorkflowYaml{}})
				}
			}
		}
	}
	return workflows, err
}