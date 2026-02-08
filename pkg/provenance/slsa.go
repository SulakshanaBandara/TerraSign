package provenance

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// SLSAProvenance represents SLSA provenance attestation
type SLSAProvenance struct {
	Type          string        `json:"_type"`
	PredicateType string        `json:"predicateType"`
	Subject       []Subject     `json:"subject"`
	Predicate     Predicate     `json:"predicate"`
}

// Subject represents the artifact being attested
type Subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// Predicate contains the provenance information
type Predicate struct {
	Builder       Builder       `json:"builder"`
	BuildType     string        `json:"buildType"`
	Invocation    Invocation    `json:"invocation"`
	Metadata      Metadata      `json:"metadata"`
	Materials     []Material    `json:"materials"`
}

// Builder identifies the build system
type Builder struct {
	ID string `json:"id"`
}

// Invocation describes how the build was invoked
type Invocation struct {
	ConfigSource ConfigSource          `json:"configSource"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	Environment  map[string]string     `json:"environment,omitempty"`
}

// ConfigSource identifies the source repository
type ConfigSource struct {
	URI       string            `json:"uri"`
	Digest    map[string]string `json:"digest"`
	EntryPoint string           `json:"entryPoint,omitempty"`
}

// Metadata contains build timing information
type Metadata struct {
	BuildStartedOn  time.Time `json:"buildStartedOn"`
	BuildFinishedOn time.Time `json:"buildFinishedOn"`
	Completeness    Completeness `json:"completeness"`
	Reproducible    bool      `json:"reproducible"`
}

// Completeness describes what information is complete
type Completeness struct {
	Parameters  bool `json:"parameters"`
	Environment bool `json:"environment"`
	Materials   bool `json:"materials"`
}

// Material represents input materials to the build
type Material struct {
	URI    string            `json:"uri"`
	Digest map[string]string `json:"digest"`
}

// ProvenanceGenerator generates SLSA provenance
type ProvenanceGenerator struct {
	builderID string
}

// NewProvenanceGenerator creates a new provenance generator
func NewProvenanceGenerator(builderID string) *ProvenanceGenerator {
	return &ProvenanceGenerator{
		builderID: builderID,
	}
}

// Generate generates SLSA provenance for a Terraform plan
func (g *ProvenanceGenerator) Generate(planPath string, buildStartTime time.Time) (*SLSAProvenance, error) {
	// Calculate plan hash
	planHash, err := calculateSHA256(planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate plan hash: %w", err)
	}

	// Get git information
	gitURI, gitCommit, err := getGitInfo()
	if err != nil {
		// Non-fatal, just log
		gitURI = "unknown"
		gitCommit = "unknown"
	}

	provenance := &SLSAProvenance{
		Type:          "https://in-toto.io/Statement/v0.1",
		PredicateType: "https://slsa.dev/provenance/v0.2",
		Subject: []Subject{
			{
				Name: planPath,
				Digest: map[string]string{
					"sha256": planHash,
				},
			},
		},
		Predicate: Predicate{
			Builder: Builder{
				ID: g.builderID,
			},
			BuildType: "https://terrasign.dev/terraform-plan@v1",
			Invocation: Invocation{
				ConfigSource: ConfigSource{
					URI: gitURI,
					Digest: map[string]string{
						"sha1": gitCommit,
					},
					EntryPoint: "terraform plan",
				},
				Environment: map[string]string{
					"TERRAFORM_VERSION": getTerraformVersion(),
				},
			},
			Metadata: Metadata{
				BuildStartedOn:  buildStartTime,
				BuildFinishedOn: time.Now(),
				Completeness: Completeness{
					Parameters:  true,
					Environment: true,
					Materials:   true,
				},
				Reproducible: false, // Terraform plans are not fully reproducible
			},
			Materials: []Material{
				{
					URI: gitURI,
					Digest: map[string]string{
						"sha1": gitCommit,
					},
				},
			},
		},
	}

	return provenance, nil
}

// Save saves provenance to disk
func (g *ProvenanceGenerator) Save(provenance *SLSAProvenance, planPath string) error {
	provenancePath := planPath + ".provenance"
	
	data, err := json.MarshalIndent(provenance, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal provenance: %w", err)
	}

	if err := os.WriteFile(provenancePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write provenance: %w", err)
	}

	return nil
}

// LoadProvenance loads provenance from disk
func LoadProvenance(planPath string) (*SLSAProvenance, error) {
	provenancePath := planPath + ".provenance"
	
	data, err := os.ReadFile(provenancePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read provenance: %w", err)
	}

	var provenance SLSAProvenance
	if err := json.Unmarshal(data, &provenance); err != nil {
		return nil, fmt.Errorf("failed to parse provenance: %w", err)
	}

	return &provenance, nil
}

// calculateSHA256 calculates SHA256 hash of a file
func calculateSHA256(filePath string) (string, error) {
	cmd := exec.Command("shasum", "-a", "256", filePath)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Output format: "hash  filename"
	parts := string(output)
	if len(parts) > 0 {
		return parts[:64], nil // SHA256 is 64 hex chars
	}

	return "", fmt.Errorf("failed to parse hash output")
}

// getGitInfo gets current git repository information
func getGitInfo() (uri string, commit string, err error) {
	// Get remote URL
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	output, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	uri = string(output)

	// Get current commit
	cmd = exec.Command("git", "rev-parse", "HEAD")
	output, err = cmd.Output()
	if err != nil {
		return uri, "", err
	}
	commit = string(output)

	return uri, commit, nil
}

// getTerraformVersion gets the Terraform version
func getTerraformVersion() string {
	cmd := exec.Command("terraform", "version", "-json")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	var versionInfo map[string]interface{}
	if err := json.Unmarshal(output, &versionInfo); err != nil {
		return "unknown"
	}

	if version, ok := versionInfo["terraform_version"].(string); ok {
		return version
	}

	return "unknown"
}
