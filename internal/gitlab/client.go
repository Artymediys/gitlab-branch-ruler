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
			if project.DefaultBranch == "" {
				continue
			}

			if err = glClient.ProtectBranch(project.ID, project.DefaultBranch); err != nil {
				log.Printf("ERROR: project: %s (ID=%d): %v", project.Name, project.ID, err)
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

// ProtectBranch sets branch protection for “branchName” to developers+maintainers
func (c *Client) ProtectBranch(projectID int, branchName string) error {
	params := url.Values{
		"name":               {branchName},
		"push_access_level":  {strconv.Itoa(c.pushAccessLevel)},
		"merge_access_level": {strconv.Itoa(c.mergeAccessLevel)},
	}

	resp, err := c.doRequest("POST", "/projects/"+strconv.Itoa(projectID)+"/protected_branches", params, nil)
	if err != nil {
		return err
	}

	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	if resp.StatusCode == 201 {
		return nil
	}
	if resp.StatusCode != 409 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	updatePath := fmt.Sprintf(
		"/projects/%d/protected_branches/%s",
		projectID,
		url.PathEscape(branchName),
	)
	resp2, err := c.doRequest("PATCH", updatePath, params, nil)
	if err != nil {
		return err
	}

	body2, _ := io.ReadAll(resp2.Body)
	defer resp2.Body.Close()

	if resp2.StatusCode >= 400 {
		return fmt.Errorf("update status %d: %s", resp2.StatusCode, string(body2))
	}

	return nil
}
