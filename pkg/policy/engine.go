package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PolicyEngine evaluates policies against Terraform plans
type PolicyEngine struct {
	policyDir string
}

// NewPolicyEngine creates a new policy engine
func NewPolicyEngine(policyDir string) *PolicyEngine {
	return &PolicyEngine{
		policyDir: policyDir,
	}
}

// PolicyViolation represents a policy violation
type PolicyViolation struct {
	Policy  string `json:"policy"`
	Message string `json:"message"`
}

// EvaluateResult contains the result of policy evaluation
type EvaluateResult struct {
	Passed     bool              `json:"passed"`
	Violations []PolicyViolation `json:"violations"`
}

// Evaluate evaluates a Terraform plan against all policies
func (p *PolicyEngine) Evaluate(planPath string) (*EvaluateResult, error) {
	// Convert plan to JSON
	planJSON, err := p.convertPlanToJSON(planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to convert plan to JSON: %w", err)
	}

	// For now, implement simple policy checks without OPA library
	// In production, you would use github.com/open-policy-agent/opa/rego
	violations := p.evaluateBuiltInPolicies(planJSON)

	result := &EvaluateResult{
		Passed:     len(violations) == 0,
		Violations: violations,
	}

	return result, nil
}

// convertPlanToJSON converts a Terraform plan to JSON format
func (p *PolicyEngine) convertPlanToJSON(planPath string) (map[string]interface{}, error) {
	// Run terraform show -json
	cmd := exec.Command("terraform", "show", "-json", planPath)
	// Do not set cmd.Dir to plan directory, as we need to run in a directory
	// where terraform is initialized (has .terraform providers)
	// cmd.Dir = filepath.Dir(planPath) 
	
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("terraform show failed: %w", err)
	}

	var planData map[string]interface{}
	if err := json.Unmarshal(output, &planData); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	return planData, nil
}

// evaluateBuiltInPolicies evaluates built-in security policies
func (p *PolicyEngine) evaluateBuiltInPolicies(planData map[string]interface{}) []PolicyViolation {
	var violations []PolicyViolation

	// Extract resource changes
	resourceChanges, ok := planData["resource_changes"].([]interface{})
	if !ok {
		return violations
	}

	for _, rc := range resourceChanges {
		resource, ok := rc.(map[string]interface{})
		if !ok {
			continue
		}

		change, _ := resource["change"].(map[string]interface{})
		actions, _ := change["actions"].([]interface{})
		
		// Skip if purely a deletion
		isDelete := false
		if len(actions) == 1 && actions[0] == "delete" {
			isDelete = true
		}
		if isDelete {
			continue
		}

		resourceType, _ := resource["type"].(string)
		after, _ := change["after"].(map[string]interface{})
		address, _ := resource["address"].(string)

		// Policy 1: No public S3 buckets
		if resourceType == "aws_s3_bucket" {
			if acl, ok := after["acl"].(string); ok {
				if strings.Contains(acl, "public") {
					violations = append(violations, PolicyViolation{
						Policy:  "no-public-s3",
						Message: fmt.Sprintf("S3 bucket '%s' has public ACL: %s", address, acl),
					})
				}
			}
		}

		// Policy 2: No overly permissive IAM policies
		if resourceType == "aws_iam_policy" {
			if policyDoc, ok := after["policy"].(string); ok {
				if strings.Contains(policyDoc, "\"*\"") && strings.Contains(policyDoc, "\"Action\"") {
					violations = append(violations, PolicyViolation{
						Policy:  "no-wildcard-iam",
						Message: fmt.Sprintf("IAM policy '%s' contains wildcard actions", address),
					})
				}
			}
		}

		// Policy 3: Security groups must not allow 0.0.0.0/0 on sensitive ports
		if resourceType == "aws_security_group" || resourceType == "aws_security_group_rule" {
			if ingress, ok := after["ingress"].([]interface{}); ok {
				for _, rule := range ingress {
					ruleMap, _ := rule.(map[string]interface{})
					cidrBlocks, _ := ruleMap["cidr_blocks"].([]interface{})
					fromPort, _ := ruleMap["from_port"].(float64)
					
					for _, cidr := range cidrBlocks {
						if cidr == "0.0.0.0/0" && (fromPort == 22 || fromPort == 3389) {
							violations = append(violations, PolicyViolation{
								Policy:  "no-public-ssh-rdp",
								Message: fmt.Sprintf("Security group '%s' allows public access to port %.0f", address, fromPort),
							})
						}
					}
				}
			}
		}

		// Policy 4: Resources must have required tags
		requiredTags := []string{"Environment", "Owner"}
		if tags, ok := after["tags"].(map[string]interface{}); ok {
			for _, reqTag := range requiredTags {
				if _, exists := tags[reqTag]; !exists {
					violations = append(violations, PolicyViolation{
						Policy:  "required-tags",
						Message: fmt.Sprintf("Resource '%s' missing required tag: %s", address, reqTag),
					})
				}
			}
		} else if resourceType != "null_resource" {
			// Only check tags for resources that support them
			violations = append(violations, PolicyViolation{
				Policy:  "required-tags",
				Message: fmt.Sprintf("Resource '%s' has no tags defined", address),
			})
		}
	}

	return violations
}

// SaveAttestation saves a policy attestation to disk
func (p *PolicyEngine) SaveAttestation(planPath string, result *EvaluateResult) error {
	attestationPath := planPath + ".policy"
	
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal attestation: %w", err)
	}

	if err := os.WriteFile(attestationPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write attestation: %w", err)
	}

	return nil
}

// LoadAttestation loads a policy attestation from disk
func LoadAttestation(planPath string) (*EvaluateResult, error) {
	attestationPath := planPath + ".policy"
	
	data, err := os.ReadFile(attestationPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read attestation: %w", err)
	}

	var result EvaluateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse attestation: %w", err)
	}

	return &result, nil
}
