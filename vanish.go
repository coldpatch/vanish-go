package vanish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client is the main API client for interacting with the Vanish Email service.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Option configures the client.
type Option func(*Client)

// WithAPIKey sets the API key for authentication.
func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// NewClient creates a new Vanish API client.
func NewClient(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Error represents an API error response.
type Error struct {
	Message    string
	StatusCode int
}

func (e *Error) Error() string {
	return fmt.Sprintf("vanish: %s (status %d)", e.Message, e.StatusCode)
}

// AttachmentMeta contains metadata about an email attachment.
type AttachmentMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Size int    `json:"size"`
}

// EmailSummary is a brief summary of an email in a mailbox listing.
type EmailSummary struct {
	ID             string    `json:"id"`
	From           string    `json:"from"`
	Subject        string    `json:"subject"`
	TextPreview    string    `json:"textPreview"`
	ReceivedAt     time.Time `json:"receivedAt"`
	HasAttachments bool      `json:"hasAttachments"`
}

// EmailDetail contains full email details including attachments.
type EmailDetail struct {
	ID             string           `json:"id"`
	From           string           `json:"from"`
	To             []string         `json:"to"`
	Subject        string           `json:"subject"`
	HTML           string           `json:"html"`
	Text           string           `json:"text"`
	ReceivedAt     time.Time        `json:"receivedAt"`
	HasAttachments bool             `json:"hasAttachments"`
	Attachments    []AttachmentMeta `json:"attachments"`
}

// PaginatedEmailList is a paginated response of emails.
type PaginatedEmailList struct {
	Data       []EmailSummary `json:"data"`
	NextCursor *string        `json:"nextCursor"`
	Total      int            `json:"total"`
}

// GenerateEmailOpts are options for generating a new email address.
type GenerateEmailOpts struct {
	Domain string `json:"domain,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

// ListEmailsOpts are options for listing emails.
type ListEmailsOpts struct {
	Limit  int
	Cursor string
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("vanish: marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("vanish: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return c.httpClient.Do(req)
}

func (c *Client) doJSON(ctx context.Context, method, path string, body, result interface{}) error {
	resp, err := c.doRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return &Error{Message: http.StatusText(resp.StatusCode), StatusCode: resp.StatusCode}
		}
		return &Error{Message: errResp.Error, StatusCode: resp.StatusCode}
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("vanish: decode response: %w", err)
		}
	}
	return nil
}

// GetDomains returns the list of available email domains.
func (c *Client) GetDomains(ctx context.Context) ([]string, error) {
	var resp struct {
		Domains []string `json:"domains"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/domains", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Domains, nil
}

// GenerateEmail creates a new unique temporary email address.
func (c *Client) GenerateEmail(ctx context.Context, opts *GenerateEmailOpts) (string, error) {
	var resp struct {
		Email string `json:"email"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/mailbox", opts, &resp); err != nil {
		return "", err
	}
	return resp.Email, nil
}

// ListEmails returns a paginated list of emails for the given mailbox address.
func (c *Client) ListEmails(ctx context.Context, address string, opts *ListEmailsOpts) (*PaginatedEmailList, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Cursor != "" {
			params.Set("cursor", opts.Cursor)
		}
	}

	path := "/mailbox/" + url.PathEscape(address)
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var result PaginatedEmailList
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetEmail retrieves the full details of an email by ID.
func (c *Client) GetEmail(ctx context.Context, emailID string) (*EmailDetail, error) {
	var result EmailDetail
	if err := c.doJSON(ctx, http.MethodGet, "/email/"+emailID, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAttachment downloads an attachment and returns its content.
func (c *Client) GetAttachment(ctx context.Context, emailID, attachmentID string) ([]byte, http.Header, error) {
	path := fmt.Sprintf("/email/%s/attachments/%s", emailID, attachmentID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return nil, nil, &Error{Message: http.StatusText(resp.StatusCode), StatusCode: resp.StatusCode}
		}
		return nil, nil, &Error{Message: errResp.Error, StatusCode: resp.StatusCode}
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("vanish: read attachment: %w", err)
	}
	return content, resp.Header, nil
}

// DeleteEmail removes an email by ID.
func (c *Client) DeleteEmail(ctx context.Context, emailID string) error {
	var resp struct {
		Success bool `json:"success"`
	}
	return c.doJSON(ctx, http.MethodDelete, "/email/"+emailID, nil, &resp)
}

// DeleteMailbox removes all emails for the given address.
func (c *Client) DeleteMailbox(ctx context.Context, address string) (int, error) {
	var resp struct {
		Deleted int `json:"deleted"`
	}
	path := "/mailbox/" + url.PathEscape(address)
	if err := c.doJSON(ctx, http.MethodDelete, path, nil, &resp); err != nil {
		return 0, err
	}
	return resp.Deleted, nil
}

// PollForEmails waits for a new email to arrive up to the given timeout.
// It returns the first new email if one arrives, or nil if timeout is reached.
func (c *Client) PollForEmails(ctx context.Context, address string, timeout, interval time.Duration, initialCount int) (*EmailSummary, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, nil
			}
			result, err := c.ListEmails(ctx, address, &ListEmailsOpts{Limit: 1})
			if err != nil {
				return nil, err
			}
			if result.Total > initialCount && len(result.Data) > 0 {
				return &result.Data[0], nil
			}
		}
	}
}
