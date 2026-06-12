package jenkins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL  string
	username string
	token    string
	http     *http.Client
}

type crumbResponse struct {
	Crumb             string `json:"crumb"`
	CrumbRequestField string `json:"crumbRequestField"`
}

type queueItemResponse struct {
	Executable *struct {
		Number int `json:"number"`
	} `json:"executable"`
	Cancelled bool   `json:"cancelled"`
	Why       string `json:"why"`
}

type buildResponse struct {
	Building bool    `json:"building"`
	Result   *string `json:"result"`
}

type lastBuildResponse struct {
	Number int `json:"number"`
}

func NewClient(baseURL, username, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		token:    token,
		http:     httpClient,
	}
}

func (c *Client) TriggerBuild(jobName string) (int, error) {
	crumb, cookies, err := c.crumb()
	if err != nil {
		return 0, err
	}

	requestURL := c.url("job", jobName, "build")
	req, err := http.NewRequest(http.MethodPost, requestURL, nil)
	if err != nil {
		return 0, err
	}
	c.authorize(req)
	if crumb.Crumb != "" {
		req.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("trigger Jenkins build: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("trigger Jenkins build returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	location := resp.Header.Get("Location")
	if location != "" {
		if number, err := c.waitForQueueExecutable(location); err == nil && number > 0 {
			return number, nil
		}
	}
	return c.getLastBuildNumber(jobName)
}

func (c *Client) GetBuildStatus(jobName string, buildNumber int) (string, error) {
	var response buildResponse
	if err := c.getJSON(c.url("job", jobName, strconv.Itoa(buildNumber), "api", "json"), &response); err != nil {
		return "", err
	}
	if response.Building || response.Result == nil || *response.Result == "" {
		return string(BuildRunning), nil
	}
	return *response.Result, nil
}

func (c *Client) GetBuildLog(jobName string, buildNumber int) (string, error) {
	req, err := http.NewRequest(http.MethodGet, c.url("job", jobName, strconv.Itoa(buildNumber), "consoleText"), nil)
	if err != nil {
		return "", err
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("get Jenkins build log: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get Jenkins build log returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20000))
	if err != nil {
		return "", fmt.Errorf("read Jenkins build log: %w", err)
	}
	return string(body), nil
}

func (c *Client) crumb() (crumbResponse, []*http.Cookie, error) {
	var crumb crumbResponse
	req, err := http.NewRequest(http.MethodGet, c.url("crumbIssuer", "api", "json"), nil)
	if err != nil {
		return crumb, nil, err
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return crumb, nil, fmt.Errorf("get Jenkins crumb: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return crumb, resp.Cookies(), nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return crumb, nil, fmt.Errorf("get Jenkins crumb returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&crumb); err != nil {
		return crumb, nil, fmt.Errorf("decode Jenkins crumb: %w", err)
	}
	return crumb, resp.Cookies(), nil
}

func (c *Client) waitForQueueExecutable(location string) (int, error) {
	apiURL := strings.TrimRight(location, "/") + "/api/json"
	for i := 0; i < 60; i++ {
		var item queueItemResponse
		if err := c.getJSON(apiURL, &item); err != nil {
			return 0, err
		}
		if item.Executable != nil && item.Executable.Number > 0 {
			return item.Executable.Number, nil
		}
		if item.Cancelled {
			return 0, fmt.Errorf("Jenkins queue item cancelled: %s", item.Why)
		}
		time.Sleep(1 * time.Second)
	}
	return 0, fmt.Errorf("timeout waiting for Jenkins queue executable")
}

func (c *Client) getLastBuildNumber(jobName string) (int, error) {
	var lastBuild lastBuildResponse
	if err := c.getJSON(c.url("job", jobName, "lastBuild", "api", "json"), &lastBuild); err != nil {
		return 0, err
	}
	if lastBuild.Number == 0 {
		return 0, fmt.Errorf("Jenkins lastBuild number is empty for job %s", jobName)
	}
	return lastBuild.Number, nil
}

func (c *Client) getJSON(requestURL string, target interface{}) error {
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	c.authorize(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Jenkins GET %s: %w", requestURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("Jenkins GET %s returned %s: %s", requestURL, resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode Jenkins response: %w", err)
	}
	return nil
}

func (c *Client) authorize(req *http.Request) {
	if c.username != "" || c.token != "" {
		req.SetBasicAuth(c.username, c.token)
	}
}

func (c *Client) url(parts ...string) string {
	parsed, err := url.Parse(c.baseURL)
	if err != nil {
		return c.baseURL
	}
	all := []string{strings.Trim(parsed.Path, "/")}
	for _, part := range parts {
		all = append(all, part)
	}
	parsed.Path = path.Join(all...)
	if !strings.HasPrefix(parsed.Path, "/") {
		parsed.Path = "/" + parsed.Path
	}
	return parsed.String()
}
