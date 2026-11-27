package v1alpha1

import (
	"encoding/json"
	"fmt"
)

const (
	nameField     = "name"
	authTypeField = "authType"
)

// MarshalJSON combines the common authentication fields and provider configuration.
func (a IdentityTypeSpec) MarshalJSON() ([]byte, error) {
	fields := map[string]any{nameField: a.Name, authTypeField: a.AuthType}
	for key, value := range a.Fields {
		if key == nameField || key == authTypeField {
			return nil, fmt.Errorf("provider field %q conflicts with a common authentication field", key)
		}
		fields[key] = value
	}
	return json.Marshal(fields)
}

// UnmarshalJSON retains provider configuration using exact, case-sensitive keys.
func (a *IdentityTypeSpec) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if fields == nil {
		return fmt.Errorf("authentication configuration must be a JSON object")
	}
	var common [2]string
	for index, key := range []string{nameField, authTypeField} {
		if raw, ok := fields[key]; ok {
			var value *string
			if err := json.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("authentication field %q must be a string: %w", key, err)
			}
			if value == nil {
				return fmt.Errorf("authentication field %q must be a string", key)
			}
			common[index] = *value
			delete(fields, key)
		}
	}
	*a = IdentityTypeSpec{Name: common[0], AuthType: IdentityType(common[1]), Fields: fields}
	return nil
}
