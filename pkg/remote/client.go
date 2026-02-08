package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client is a client for the signing service
type Client struct {
	baseURL string
	client  *http.Client
}

// NewClient creates a new signing service client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// SubmitPlan submits a plan for review
func (c *Client) SubmitPlan(planPath, submitter string) (string, error) {
	file, err := os.Open(planPath)
	if err != nil {
		return "", fmt.Errorf("failed to open plan file: %w", err)
	}
	defer file.Close()

	req, err := http.NewRequest("POST", c.baseURL+"/submit?submitter="+submitter, file)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to submit plan: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("server error: %s", string(body))
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result["id"], nil
}

// GetStatus gets the status of a submission
func (c *Client) GetStatus(id string) (*PlanSubmission, error) {
	resp, err := c.client.Get(c.baseURL + "/status/" + id)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("submission not found")
	}

	var submission PlanSubmission
	if err := json.NewDecoder(resp.Body).Decode(&submission); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &submission, nil
}

// WaitForSignature polls until the plan is signed or timeout
func (c *Client) WaitForSignature(id string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			submission, err := c.GetStatus(id)
			if err != nil {
				return err
			}

			if submission.Status == "approved" {
				return nil
			}

			if submission.Status == "rejected" {
				return fmt.Errorf("plan was rejected by admin")
			}

		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("timeout waiting for signature")
		}
	}
}

// DownloadPlan downloads the plan file
func (c *Client) DownloadPlan(id, outputPath string) error {
	return c.downloadFile(id, "plan", outputPath)
}

// DownloadSignature downloads the signature file
func (c *Client) DownloadSignature(id, outputPath string) error {
	return c.downloadFile(id, "signature", outputPath)
}

// downloadFile downloads a file from the service
func (c *Client) downloadFile(id, fileType, outputPath string) error {
	url := fmt.Sprintf("%s/download/%s/%s", c.baseURL, id, fileType)
	resp, err := c.client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("file not found")
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ListPending lists all pending submissions
func (c *Client) ListPending() ([]*PlanSubmission, error) {
	resp, err := c.client.Get(c.baseURL + "/list-pending")
	if err != nil {
		return nil, fmt.Errorf("failed to list pending: %w", err)
	}
	defer resp.Body.Close()

	var submissions []*PlanSubmission
	if err := json.NewDecoder(resp.Body).Decode(&submissions); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return submissions, nil
}

// UploadSignature uploads a signature file for a submission
func (c *Client) UploadSignature(id, signaturePath string) error {
	file, err := os.Open(signaturePath)
	if err != nil {
		return fmt.Errorf("failed to open signature file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read signature: %w", err)
	}

	url := fmt.Sprintf("%s/upload-signature/%s", c.baseURL, id)
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload signature: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}

	return nil
}
