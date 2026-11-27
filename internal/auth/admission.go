package auth

import (
	"encoding/json"
	"errors"
	"fmt"
)

// validateAdmission checks the package envelope only. CEL semantics, matching
// scope, API-server validation and installation belong to the admission registrar.
func validateAdmission(data []byte) error {
	documents, err := yamlDocuments(data)
	if err != nil {
		return err
	}
	var policyName, boundPolicy string
	var policies, bindings int
	for _, raw := range documents {
		var resource struct {
			APIVersion string `json:"apiVersion"`
			Kind       string `json:"kind"`
			Metadata   struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				PolicyName        string   `json:"policyName"`
				ValidationActions []string `json:"validationActions"`
			} `json:"spec"`
		}
		if err := json.Unmarshal(raw, &resource); err != nil {
			return err
		}
		if resource.APIVersion != "admissionregistration.k8s.io/v1" || resource.Metadata.Name == "" {
			return errors.New("resources require admissionregistration.k8s.io/v1 and metadata.name")
		}
		switch resource.Kind {
		case "ValidatingAdmissionPolicy":
			policies++
			policyName = resource.Metadata.Name
		case "ValidatingAdmissionPolicyBinding":
			bindings++
			boundPolicy = resource.Spec.PolicyName
			actions := make(map[string]bool)
			for _, action := range resource.Spec.ValidationActions {
				if actions[action] || (action != "Deny" && action != "Audit") {
					return errors.New("binding actions must be unique and use Deny with optional Audit")
				}
				actions[action] = true
			}
			if !actions["Deny"] {
				return errors.New("binding requires Deny enforcement")
			}
		default:
			return fmt.Errorf("unsupported resource kind %q", resource.Kind)
		}
	}
	if policies != 1 || bindings != 1 || boundPolicy != policyName {
		return errors.New("requires one ValidatingAdmissionPolicy and one binding referencing it")
	}
	return nil
}
