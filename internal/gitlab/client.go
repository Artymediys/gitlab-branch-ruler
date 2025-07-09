package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

// Client GitLab API wrapper
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client

	pushAccessLevel  int
	mergeAccessLevel int
}

// NewClient creates new client
func NewClient(baseURL, token string, pushLevel, mergeLevel int) *Client {
	return &Client{
		baseURL:          baseURL + "/api/v4",
		token:            token,
		httpClient:       &http.Client{},
		pushAccessLevel:  pushLevel,
		mergeAccessLevel: mergeLevel,
	}
}

func (c *Client) doRequest(method, path string, params url.Values, body io.Reader) (*http.Response, error) {
	uri := c.baseURL + path
	if params != nil {
		uri += "?" + params.Encode()
	}
	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Private-Token", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return c.httpClient.Do(req)
}

// Structs for JSON decoding
type Group struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Project struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch"`
}

// GetGroup returns group details by ID or URL-encoded path
func (c *Client) GetGroup(idOrPath string) (*Group, error) {
	resp, err := c.doRequest("GET", "/groups/"+url.PathEscape(idOrPath), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var group Group
	if err = json.NewDecoder(resp.Body).Decode(&group); err != nil {
		return nil, err
	}

	return &group, nil
}

// ProcessGroup iterates through projects and subgroups
func ProcessGroup(glClient *Client, groupID, groupName string) {
	projectList, err := glClient.ListProjects(groupID)
	if err != nil {
		log.Printf("ERROR: get projects of group: %s (ID=%s): %v", groupName, groupID, err)
	} else {
		for _, project := range projectList {
			protectedBranches := []string{"main", "master"}

			if project.DefaultBranch != "" && project.DefaultBranch != "main" && project.DefaultBranch != "master" {
				protectedBranches = append([]string{project.DefaultBranch}, protectedBranches...)
			}

			for _, branch := range protectedBranches {
				isBranchExists, err := glClient.BranchExists(project.ID, branch)
				if err != nil {
					log.Printf("ERROR: check branch: %s in project: %s (ID=%d): %v", branch, project.Name, project.ID, err)
					continue
				}

				if !isBranchExists {
					continue
				}

				if err = glClient.EnsureBranchProtection(project.ID, branch); err != nil {
					log.Printf("ERROR: protect branch: %s in project: %s (ID=%d): %v", branch, project.Name, project.ID, err)
				}
			}
		}
	}

	subgroupList, err := glClient.ListSubgroups(groupID)
	if err != nil {
		log.Printf("ERROR: get subgroups of group: %s (ID=%s): %v", groupName, groupID, err)
		return
	}

	for _, subgroup := range subgroupList {
		log.Printf("Entering subgroup: %s (ID=%d)", subgroup.Name, subgroup.ID)
		ProcessGroup(glClient, strconv.Itoa(subgroup.ID), subgroup.Name)
	}
}

// ListSubgroups returns all subgroups
func (c *Client) ListSubgroups(groupID string) ([]Group, error) {
	var allSubgroups []Group
	page := 1
	for {
		params := url.Values{"per_page": {"100"}, "page": {strconv.Itoa(page)}}
		resp, err := c.doRequest("GET", "/groups/"+url.PathEscape(groupID)+"/subgroups", params, nil)
		if err != nil {
			return nil, err
		}

		var pageSubgroups []Group
		if err = json.NewDecoder(resp.Body).Decode(&pageSubgroups); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		if len(pageSubgroups) == 0 {
			break
		}

		allSubgroups = append(allSubgroups, pageSubgroups...)

		if resp.Header.Get("X-Next-Page") == "" {
			break
		}

		page++
	}

	return allSubgroups, nil
}

// ListProjects returns all group projects
func (c *Client) ListProjects(groupID string) ([]Project, error) {
	var allProjects []Project
	page := 1
	for {
		params := url.Values{"per_page": {"100"}, "page": {strconv.Itoa(page)}}
		resp, err := c.doRequest("GET", "/groups/"+url.PathEscape(groupID)+"/projects", params, nil)
		if err != nil {
			return nil, err
		}

		var pageProjects []Project
		if err = json.NewDecoder(resp.Body).Decode(&pageProjects); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		if len(pageProjects) == 0 {
			break
		}

		allProjects = append(allProjects, pageProjects...)

		if resp.Header.Get("X-Next-Page") == "" {
			break
		}

		page++
	}

	return allProjects, nil
}

// BranchExists checks if project has “branchName”
func (c *Client) BranchExists(projectID int, branchName string) (bool, error) {
	reqURL := fmt.Sprintf("%s/projects/%d/repository/branches/%s", c.baseURL, projectID, url.PathEscape(branchName))
	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("Private-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("status %d: %s", resp.StatusCode, body)
}

// EnsureBranchProtection sets branch protection for “branchName”
func (c *Client) EnsureBranchProtection(projectID int, branchName string) error {
	params := url.Values{
		"name":               {branchName},
		"push_access_level":  {strconv.Itoa(c.pushAccessLevel)},
		"merge_access_level": {strconv.Itoa(c.mergeAccessLevel)},
	}

	postPath := fmt.Sprintf("/projects/%d/protected_branches", projectID)
	resp, err := c.doRequest("POST", postPath, params, nil)
	if err != nil {
		return err
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		return nil
	}
	if resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("create failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	deletePath := fmt.Sprintf("/projects/%d/protected_branches/%s",
		projectID, url.PathEscape(branchName))
	resp, err = c.doRequest("DELETE", deletePath, nil, nil)
	if err != nil {
		return err
	}
	respBody, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("delete failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	resp, err = c.doRequest("POST", postPath, params, nil)
	if err != nil {
		return err
	}
	respBody, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("re-create failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
