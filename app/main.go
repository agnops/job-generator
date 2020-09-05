package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/gorilla/handlers"
	"gopkg.in/go-playground/webhooks.v5/github"
	"gopkg.in/go-playground/webhooks.v5/gitlab"
)

var scmProvider = os.Getenv("scmProvider")
var cloudName = os.Getenv("cloudName")

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	io.WriteString(w, `{"healthy": true}`)
}

func getScmWebhookSecret() string {
	var helmRelease = os.Getenv("HELM_RELEASE")
	secretName := strings.ToLower(helmRelease) + "-" + scmProvider + "-agnops-webhook-secret"
	return getWebhookSecret(secretName)
}

func GitHubWebhooks(w http.ResponseWriter, r *http.Request) {

	hook, _ := github.New(github.Options.Secret(getScmWebhookSecret()))

	payload, err := hook.Parse(r, github.PullRequestEvent, github.PushEvent)
	if err != nil {
		if err == github.ErrEventNotFound {
			// ok event wasn;t one of the ones asked to be parsed
		}
	}
	switch payload.(type) {

	case github.PullRequestPayload:
		pullRequest := payload.(github.PullRequestPayload)
		fmt.Printf("%+v", pullRequest)

	case github.PushPayload:
		pushPl := payload.(github.PushPayload)

		orgOrUserName := GetOwnerOrRepositoryName(pushPl.Repository.Owner.HTMLURL)
		gitRepository := GetOwnerOrRepositoryName(pushPl.Repository.HTMLURL)
		oauthToken, _ := GetUserOrOrganizationToken(scmProvider, orgOrUserName)

		for _, commit := range pushPl.Commits {

			changedFiles := append(commit.Added, commit.Modified...)
			branch := strings.Replace(pushPl.Ref, "refs/heads/", "", -1)
			workflows, err := checkGitWorkflowExistInRepo(pushPl.Repository.CloneURL, orgOrUserName, gitRepository, pushPl.HeadCommit.ID, oauthToken, "x-oauth-basic", changedFiles, branch)

			log.Println(err)

			if err == nil && len(workflows) > 0 {

				for i, workflow := range workflows {
					scmWorkflowDetails := &ScmWorkflowDetails{
						ScProvider:    "GitHub",
						GitOrgProject: orgOrUserName,
						GitRepository: gitRepository,
						OAuthToken:    oauthToken,
						CloneURL:      pushPl.Repository.CloneURL,
						Branch:        branch,
						CommitId:      commit.ID,
						CommitMsg:     commit.Message,
						CommitUrl:     commit.URL,
						Email:         commit.Author.Email,
						Workflow:      workflow,
					}
					if !reflect.DeepEqual(WorkflowYaml{}, workflow.WorkflowYaml) {
						createJobObject(scmWorkflowDetails, i)
					} else {
						createConfigMap(scmWorkflowDetails, i)
					}
				}
			}
		}
	}
}

func GitLabWebhooks(w http.ResponseWriter, r *http.Request) {

	hook, _ := gitlab.New(gitlab.Options.Secret(getScmWebhookSecret()))

	payload, err := hook.Parse(r, gitlab.PushEvents)
	if err != nil {
		if err == gitlab.ErrEventNotFound {
			log.Println("ok event wasn`t one of the ones asked to be parsed")
		}
	}
	switch payload.(type) {

	case gitlab.PushEventPayload:
		log.Println("PushEventPayload")
		pushPl := payload.(gitlab.PushEventPayload)

		orgOrUserName := pushPl.UserUsername
		gitRepository := pushPl.Repository.Name
		oauthToken, _ := GetUserOrOrganizationToken(scmProvider, orgOrUserName)

		for _, commit := range pushPl.Commits {

			changedFiles := append(commit.Added, commit.Modified...)
			branch := strings.Replace(pushPl.Ref, "refs/heads/", "", -1)
			workflows, err := checkGitWorkflowExistInRepo(pushPl.Project.GitHTTPURL, orgOrUserName, gitRepository, commit.ID, oauthToken, "oauth2", changedFiles, branch)

			log.Println(err)

			if err == nil && len(workflows) > 0 {

				for i, workflow := range workflows {

					scmWorkflowDetails := &ScmWorkflowDetails{
						ScProvider:    "GitLab",
						GitOrgProject: orgOrUserName,
						GitRepository: gitRepository,
						OAuthToken:    oauthToken,
						CloneURL:      pushPl.Project.GitHTTPURL,
						Branch:        branch,
						CommitId:      commit.ID,
						CommitMsg:     commit.Message,
						CommitUrl:     commit.URL,
						Email:         commit.Author.Email,
						Workflow:      workflow,
					}
					if !reflect.DeepEqual(WorkflowYaml{}, workflow.WorkflowYaml) {
						createJobObject(scmWorkflowDetails, i)
					} else {
						createConfigMap(scmWorkflowDetails, i)
					}
				}
			}
		}
	}
}

func main() {

	initK8sClientset()

	switch scmProvider {
	case "github":
		http.HandleFunc("/webhooks", GitHubWebhooks)
	case "gitlab":
		http.HandleFunc("/webhooks", GitLabWebhooks)
	}

	http.HandleFunc("/healthcheck", HealthCheckHandler)

	http.ListenAndServe(":3000", handlers.LoggingHandler(os.Stdout, http.DefaultServeMux))
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Println("%s: %s", msg, err)
	}
}
