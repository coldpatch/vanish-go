# Vanish Client for Go

A lightweight, zero-dependency Go client for the [Vanish](https://github.com/coldpatch/vanish) temporary email service API.

## Requirements

- Go 1.21+

## Installation

```bash
go get github.com/coldpatch/vanish-go
```

Or copy `vanish.go` directly into your project.

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/coldpatch/vanish-go"
)

func main() {
    // Create client with optional API key
    client := vanish.NewClient(
        "https://api.vanish.host",
        vanish.WithAPIKey("your-key"),
        vanish.WithTimeout(30*time.Second),
    )

    ctx := context.Background()

    // Generate a temporary email address
    email, err := client.GenerateEmail(ctx, nil)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Your temp email: %s\n", email)

    // Or with options
    email, err = client.GenerateEmail(ctx, &vanish.GenerateEmailOpts{
        Domain: "vanish.host",
        Prefix: "mytest",
    })

    // List emails in the mailbox
    result, err := client.ListEmails(ctx, email, &vanish.ListEmailsOpts{Limit: 20})
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Total emails: %d\n", result.Total)

    for _, summary := range result.Data {
        fmt.Printf("  - %s from %s\n", summary.Subject, summary.From)
    }

    // Get full email details
    if len(result.Data) > 0 {
        detail, err := client.GetEmail(ctx, result.Data[0].ID)
        if err != nil {
            log.Fatal(err)
        }
        fmt.Printf("HTML: %s\n", detail.HTML)
        fmt.Printf("Text: %s\n", detail.Text)

        // Download attachments
        for _, att := range detail.Attachments {
            content, headers, err := client.GetAttachment(ctx, detail.ID, att.ID)
            if err != nil {
                log.Fatal(err)
            }
            fmt.Printf("Downloaded %s (%d bytes)\n", att.Name, len(content))
            fmt.Printf("Content-Type: %s\n", headers.Get("Content-Type"))
        }
    }
}
```

## Polling for New Emails

Wait for an email to arrive with the built-in polling utility:

```go
// Wait up to 60 seconds for a new email
newEmail, err := client.PollForEmails(
    ctx,
    email,
    time.Minute,      // timeout
    5*time.Second,    // check interval
    0,                // initial count to compare against
)
if err != nil {
    log.Fatal(err)
}

if newEmail != nil {
    fmt.Printf("New email received: %s\n", newEmail.Subject)
} else {
    fmt.Println("No email received within timeout")
}
```

## API Reference

### Client Creation

```go
client := vanish.NewClient(baseURL string, opts ...Option)
```

#### Options

| Option                                | Description                    |
| ------------------------------------- | ------------------------------ |
| `WithAPIKey(key string)`              | Set API key for authentication |
| `WithHTTPClient(client *http.Client)` | Use custom HTTP client         |
| `WithTimeout(timeout time.Duration)`  | Set request timeout            |

### Methods

All methods accept a `context.Context` as the first parameter for cancellation and timeouts.

#### `GetDomains(ctx) ([]string, error)`

Returns list of available email domains.

#### `GenerateEmail(ctx, opts *GenerateEmailOpts) (string, error)`

Generate a unique temporary email address.

#### `ListEmails(ctx, address string, opts *ListEmailsOpts) (*PaginatedEmailList, error)`

List emails for a mailbox with pagination support.

#### `GetEmail(ctx, emailID string) (*EmailDetail, error)`

Get full details of a specific email including attachments.

#### `GetAttachment(ctx, emailID, attachmentID string) ([]byte, http.Header, error)`

Download an attachment. Returns content bytes and HTTP headers.

#### `DeleteEmail(ctx, emailID string) error`

Delete a specific email.

#### `DeleteMailbox(ctx, address string) (int, error)`

Delete all emails in a mailbox. Returns count of deleted emails.

#### `PollForEmails(ctx, address string, timeout, interval time.Duration, initialCount int) (*EmailSummary, error)`

Poll for new emails until one arrives or timeout.

## Types

### `EmailSummary`

```go
type EmailSummary struct {
    ID             string    `json:"id"`
    From           string    `json:"from"`
    Subject        string    `json:"subject"`
    TextPreview    string    `json:"textPreview"`
    ReceivedAt     time.Time `json:"receivedAt"`
    HasAttachments bool      `json:"hasAttachments"`
}
```

### `EmailDetail`

```go
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
```

### `AttachmentMeta`

```go
type AttachmentMeta struct {
    ID   string `json:"id"`
    Name string `json:"name"`
    Type string `json:"type"`
    Size int    `json:"size"`
}
```

### `PaginatedEmailList`

```go
type PaginatedEmailList struct {
    Data       []EmailSummary `json:"data"`
    NextCursor *string        `json:"nextCursor"`
    Total      int            `json:"total"`
}
```

## Error Handling

```go
import "errors"

email, err := client.GetEmail(ctx, "invalid-id")
if err != nil {
    var vanishErr *vanish.Error
    if errors.As(err, &vanishErr) {
        fmt.Printf("API Error: %s (status %d)\n", vanishErr.Message, vanishErr.StatusCode)
    } else {
        fmt.Printf("Other error: %v\n", err)
    }
}
```

## Context Support

All methods support context for cancellation and timeouts:

```go
// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

email, err := client.GenerateEmail(ctx, nil)

// With cancellation
ctx, cancel := context.WithCancel(context.Background())
go func() {
    time.Sleep(5 * time.Second)
    cancel()
}()

newEmail, err := client.PollForEmails(ctx, email, time.Minute, time.Second, 0)
if errors.Is(err, context.Canceled) {
    fmt.Println("Polling was cancelled")
}
```

## License

MIT
