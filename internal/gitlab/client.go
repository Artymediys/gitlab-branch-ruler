package gitlab

import (
	"bytes"
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

			if err = glClient.EnsureBranchProtection(project.ID, project.DefaultBranch); err != nil {
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

// Structs for JSON encoding
type protectPayload struct {
	Name           string              `json:"name,omitempty"`
	AllowedToPush  []accessLevelHolder `json:"allowed_to_push"`
	AllowedToMerge []accessLevelHolder `json:"allowed_to_merge"`
}

type accessLevelHolder struct {
	AccessLevel int `json:"access_level"`
}

// EnsureBranchProtection sets branch protection for “branchName”
func (c *Client) EnsureBranchProtection(projectID int, branchName string) error {
	payload := protectPayload{
		Name:           branchName,
		AllowedToPush:  []accessLevelHolder{{c.pushAccessLevel}},
		AllowedToMerge: []accessLevelHolder{{c.mergeAccessLevel}},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	postURL := fmt.Sprintf("%s/projects/%d/protected_branches", c.baseURL, projectID)
	patchURL := fmt.Sprintf("%s/projects/%d/protected_branches/%s", c.baseURL, projectID, url.PathEscape(branchName))

	doJSON := func(method, url string, data []byte) (int, []byte, error) {
		req, err := http.NewRequest(method, url, bytes.NewReader(data))
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Private-Token", c.token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return 0, nil, err
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		return resp.StatusCode, respBody, nil
	}

	status, respBody, err := doJSON("POST", postURL, bodyBytes)
	if err != nil {
		return err
	}

	switch status {
	case http.StatusCreated:
		return nil
	case http.StatusConflict:
	default:
		return fmt.Errorf("create %d: %s", status, respBody)
	}

	status, respBody, err = doJSON("PATCH", patchURL, bodyBytes)
	if err != nil {
		return err
	}
	if status < 400 {
		ok, err := c.protectionHasLevel(projectID, branchName, c.pushAccessLevel)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}

	// 4) если PATCH вернул ошибку или уровни не обновились — удаляем и создаём заново
	return c.deleteAndRecreateProtection(postURL, patchURL, bodyBytes)
}

// protectionHasLevel ensure a branch has access level
func (c *Client) protectionHasLevel(projectID int, branchName string, level int) (bool, error) {
	getURL := fmt.Sprintf("%s/projects/%d/protected_branches/%s", c.baseURL, projectID, url.PathEscape(branchName))
	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Private-Token", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}

	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	pattern := fmt.Sprintf(`"access_level":%d`, level)
	return bytes.Contains(data, []byte(pattern)), nil
}

// deleteAndRecreateProtection hard recreate branch protection
func (c *Client) deleteAndRecreateProtection(postURL, delURL string, bodyBytes []byte) error {
	req, _ := http.NewRequest("DELETE", delURL, nil)
	req.Header.Set("Private-Token", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("delete %d: %s", resp.StatusCode, data)
	}

	req2, _ := http.NewRequest("POST", postURL, bytes.NewReader(bodyBytes))
	req2.Header.Set("Private-Token", c.token)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := c.httpClient.Do(req2)
	if err != nil {
		return err
	}

	data2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if resp2.StatusCode >= 400 {
		return fmt.Errorf("re-create %d: %s", resp2.StatusCode, data2)
	}

	return nil
}
