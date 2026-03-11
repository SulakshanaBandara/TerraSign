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

// BaseURL returns the service base URL
func (c *Client) BaseURL() string {
	return c.baseURL
}

// SubmitPlan submits a plan for review
func (c *Client) SubmitPlan(planPath, submitter string, threshold int) (string, error) {
	file, err := os.Open(planPath)
	if err != nil {
		return "", fmt.Errorf("failed to open plan file: %w", err)
	}
	defer file.Close()

	url := fmt.Sprintf("%s/submit?submitter=%s&threshold=%d", c.baseURL, submitter, threshold)
	req, err := http.NewRequest("POST", url, file)
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

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	id, _ := result["id"].(string)
	return id, nil
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

// DownloadBundle downloads the cosign bundle file
func (c *Client) DownloadBundle(id, outputPath string) error {
	return c.downloadFile(id, "bundle", outputPath)
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

// UploadBundle uploads a cosign bundle file for a submission
func (c *Client) UploadBundle(id, bundlePath string) error {
	return c.UploadBundleForApprover(id, bundlePath, "admin", "")
}

// UploadBundleForApprover uploads a bundle file and records the approval under the given reviewer name
func (c *Client) UploadBundleForApprover(id, bundlePath, approver, keyHint string) error {
	file, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to open bundle file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read bundle: %w", err)
	}

	url := fmt.Sprintf("%s/upload-bundle/%s?approver=%s&key_hint=%s", c.baseURL, id, approver, keyHint)
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload bundle: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}

	return nil
}

// GetApprovals retrieves the multi-party approval status for a submission
func (c *Client) GetApprovals(id string) (*PlanSubmission, error) {
	resp, err := c.client.Get(fmt.Sprintf("%s/approvals/%s", c.baseURL, id))
	if err != nil {
		return nil, fmt.Errorf("failed to get approvals: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("submission not found")
	}

	var submission PlanSubmission
	if err := json.NewDecoder(resp.Body).Decode(&submission); err != nil {
		return nil, fmt.Errorf("failed to parse approvals response: %w", err)
	}
	return &submission, nil
}

// SetLockdown enables or disables emergency lockdown.
// identity, hostname, and reason are recorded in the server audit log.
func (c *Client) SetLockdown(enable bool, identity, hostname, reason string) error {
	status := "off"
	if enable {
		status = "on"
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/lockdown?mode=%s", c.baseURL, status),
		nil,
	)
	if err != nil {
		return err
	}

	// Send GPG-verified identity so the server can record it
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Terrasign-Identity", identity)
	req.Header.Set("X-Terrasign-Hostname", hostname)
	req.Header.Set("X-Terrasign-Reason", reason)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned error: %s", string(body))
	}

	return nil
}

// RejectSubmission rejects a pending plan submission
func (c *Client) RejectSubmission(id, reviewer, reason string) error {
	url := fmt.Sprintf("%s/reject/%s?reviewer=%s&reason=%s", c.baseURL, id, reviewer, reason)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reject submission: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}

	return nil
}

// GetPolicy retrieves the current global approval threshold policy from the server
func (c *Client) GetPolicy() (*GlobalPolicy, error) {
	resp, err := c.client.Get(c.baseURL + "/policy")
	if err != nil {
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}

	var policy GlobalPolicy
	if err := json.NewDecoder(resp.Body).Decode(&policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy: %w", err)
	}
	return &policy, nil
}

// SetPolicy updates the server-wide approval threshold policy
func (c *Client) SetPolicy(threshold int, setBy, reason string) (*GlobalPolicy, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"threshold": threshold,
		"set_by":    setBy,
		"reason":    reason,
	})
	resp, err := c.client.Post(c.baseURL+"/policy", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to set policy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error: %s", string(errBody))
	}

	var policy GlobalPolicy
	if err := json.NewDecoder(resp.Body).Decode(&policy); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &policy, nil
}
