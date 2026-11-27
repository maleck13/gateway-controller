package v1alpha1

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAuthenticationJSONPreservesProviderFields(t *testing.T) {
	const input = `{"name":"consumers","authType":"custom","Name":"provider-name","AuthType":false,"nested":{"values":[null,true,9007199254740993]},"empty":null}`
	var auth IdentityTypeSpec
	if err := json.Unmarshal([]byte(input), &auth); err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(auth)
	if err != nil {
		t.Fatal(err)
	}
	var wantFields, gotFields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(input), &wantFields); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &gotFields); err != nil {
		t.Fatal(err)
	}
	if len(gotFields) != len(wantFields) {
		t.Fatalf("fields lost: got %s, want %s", got, input)
	}
	for key, want := range wantFields {
		if string(gotFields[key]) != string(want) {
			t.Errorf("field %q: got %s, want %s", key, gotFields[key], want)
		}
	}
}

func TestAuthenticationJSONRejectsInvalidCommonFields(t *testing.T) {
	for _, input := range []string{
		`null`, `[]`, `true`, `{"name":null}`, `{"name":1}`, `{"name":false}`,
		`{"authType":null}`, `{"authType":{}}`, `{"authType":[]}`, `{"name":`,
	} {
		t.Run(input, func(t *testing.T) {
			var auth IdentityTypeSpec
			if err := json.Unmarshal([]byte(input), &auth); err == nil {
				t.Fatal("invalid authentication JSON accepted")
			}
		})
	}
}

func TestAuthenticationJSONRejectsCommonFieldCollisions(t *testing.T) {
	for _, key := range []string{"name", "authType"} {
		t.Run(key, func(t *testing.T) {
			auth := IdentityTypeSpec{Fields: map[string]json.RawMessage{key: json.RawMessage(`"override"`)}}
			if _, err := json.Marshal(auth); err == nil {
				t.Fatal("conflicting provider field accepted")
			}
		})
	}
}

func TestAuthenticationSurvivesKubernetesConversion(t *testing.T) {
	const input = `{"spec":{"governance":{"authentication":[{"name":"consumers","authType":"custom","keySource":{"secretRef":{"name":"keys","key":"keys.yaml"}},"nested":[null,true,9007199254740993],"Name":"provider","AuthType":false}]}}}`
	var original AIGateway
	if err := json.Unmarshal([]byte(input), &original); err != nil {
		t.Fatal(err)
	}
	converted, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&original)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip AIGateway
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(converted, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(authenticationJSONValue(t, original.Spec), authenticationJSONValue(t, roundTrip.Spec)) {
		t.Fatalf("provider fields changed during conversion: got %+v, want %+v", roundTrip.Spec.Governance, original.Spec.Governance)
	}
	copy := original.DeepCopy()
	copy.Spec.Governance.Authentication[0].Fields["nested"][0] = '0'
	delete(copy.Spec.Governance.Authentication[0].Fields, "keySource")
	if string(original.Spec.Governance.Authentication[0].Fields["nested"]) != `[null,true,9007199254740993]` {
		t.Fatal("DeepCopy shares provider byte slices")
	}
	if _, ok := original.Spec.Governance.Authentication[0].Fields["keySource"]; !ok {
		t.Fatal("DeepCopy shares provider maps")
	}
	patch := client.MergeFrom(original.DeepCopy())
	original.Spec.Governance.Authentication[0].Fields["nested"] = json.RawMessage(`[9007199254740995]`)
	data, err := patch.Data(&original)
	if err != nil {
		t.Fatal(err)
	}
	var changed map[string]json.RawMessage
	if err := json.Unmarshal(data, &changed); err != nil {
		t.Fatal(err)
	}
	if len(changed["spec"]) == 0 {
		t.Fatalf("MergeFrom lost provider change: %s", data)
	}
}

func authenticationJSONValue(t *testing.T, value any) any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var result any
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}
