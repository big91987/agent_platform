package platform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// WorkflowDelivery reads external facts for a PR already created by this run.
// It never changes GitHub or treats a draft PR as a deployment.
type WorkflowDelivery struct {
	PullRequestURL string               `json:"pull_request_url"`
	RunHeadSHA     string               `json:"run_head_sha"`
	CurrentHeadSHA string               `json:"current_head_sha"`
	State          string               `json:"state"`
	Draft          bool                 `json:"draft"`
	Merged         bool                 `json:"merged"`
	MainSHA        string               `json:"main_sha,omitempty"`
	ActionsURL     string               `json:"actions_url"`
	Deployments    []WorkflowDeployment `json:"deployments"`
}

type WorkflowDeployment struct {
	Environment string `json:"environment"`
	State       string `json:"state"`
	Version     string `json:"version"`
	URL         string `json:"url,omitempty"`
	LogURL      string `json:"log_url,omitempty"`
}

func (h *Server) workflowDelivery(ctx context.Context, c Caller, id string) (WorkflowDelivery, error) {
	run, err := h.store.WorkflowRun(c, id)
	if err != nil {
		return WorkflowDelivery{}, err
	}
	var frozen Connector
	var receipt *ConnectorReceipt
	for _, step := range run.Steps {
		if step.Receipt == nil || step.Receipt.Kind != "github.pull_request" {
			continue
		}
		node := run.Definition.node(step.NodeID)
		frozen = run.Connectors[node.ConnectorID]
		receipt = step.Receipt
	}
	if receipt == nil || receipt.Number < 1 || frozen.Kind != "github.pull_request" {
		return WorkflowDelivery{}, ErrNotFound
	}
	current, err := h.store.Connector(c, frozen.ID)
	if err != nil {
		return WorkflowDelivery{}, err
	}
	if !current.Enabled || current.Kind != frozen.Kind || current.Repository != frozen.Repository || current.TokenEnv != frozen.TokenEnv {
		return WorkflowDelivery{}, ErrForbidden
	}
	path := "/repos/" + frozen.Repository
	var pr struct {
		HTMLURL        string `json:"html_url"`
		State          string `json:"state"`
		Draft          bool   `json:"draft"`
		Merged         bool   `json:"merged"`
		MergeCommitSHA string `json:"merge_commit_sha"`
		Head           struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := githubRequest(ctx, current, http.MethodGet, path+"/pulls/"+strconv.Itoa(receipt.Number), nil, &pr); err != nil {
		return WorkflowDelivery{}, err
	}
	wantURL := fmt.Sprintf("https://github.com/%s/pull/%d", frozen.Repository, receipt.Number)
	if !strings.EqualFold(pr.HTMLURL, wantURL) || !strings.EqualFold(receipt.URL, wantURL) {
		return WorkflowDelivery{}, errors.New("GitHub PR identity differs from the saved run receipt")
	}
	result := WorkflowDelivery{
		PullRequestURL: pr.HTMLURL,
		RunHeadSHA:     receipt.HeadSHA,
		CurrentHeadSHA: pr.Head.SHA,
		State:          pr.State,
		Draft:          pr.Draft,
		Merged:         pr.Merged,
		ActionsURL:     "https://github.com/" + frozen.Repository + "/actions",
		Deployments:    []WorkflowDeployment{},
	}
	if !pr.Merged {
		return result, nil
	}
	if !validGitSHA(pr.MergeCommitSHA) {
		return WorkflowDelivery{}, errors.New("merged PR has no valid merge commit SHA")
	}
	result.MainSHA = pr.MergeCommitSHA
	var deployments []struct {
		ID          int64  `json:"id"`
		SHA         string `json:"sha"`
		Environment string `json:"environment"`
	}
	query := path + "/deployments?sha=" + url.QueryEscape(result.MainSHA) + "&per_page=100"
	if err := githubRequest(ctx, current, http.MethodGet, query, nil, &deployments); err != nil {
		return WorkflowDelivery{}, err
	}
	if len(deployments) == 100 {
		return WorkflowDelivery{}, errors.New("deployment history exceeds one page; inspect GitHub Actions")
	}
	for _, deployment := range deployments {
		if deployment.SHA != result.MainSHA || deployment.ID < 1 {
			return WorkflowDelivery{}, errors.New("GitHub deployment does not match merged main commit")
		}
		item := WorkflowDeployment{Environment: deployment.Environment, Version: deployment.SHA, State: "pending"}
		var statuses []struct {
			State          string `json:"state"`
			EnvironmentURL string `json:"environment_url"`
			LogURL         string `json:"log_url"`
		}
		statusPath := fmt.Sprintf("%s/deployments/%d/statuses?per_page=1", path, deployment.ID)
		if err := githubRequest(ctx, current, http.MethodGet, statusPath, nil, &statuses); err != nil {
			return WorkflowDelivery{}, err
		}
		if len(statuses) > 0 {
			item.State = statuses[0].State
			item.URL = statuses[0].EnvironmentURL
			item.LogURL = statuses[0].LogURL
		}
		result.Deployments = append(result.Deployments, item)
	}
	return result, nil
}

func validGitSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, c := range value {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return false
		}
	}
	return true
}
