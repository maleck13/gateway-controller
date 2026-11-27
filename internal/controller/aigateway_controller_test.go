package controller

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	aigatewayv1alpha1 "github.com/praxis-proxy/gateway-controller/api/aigateway/v1alpha1"
)

func TestIsTenantReady(t *testing.T) {
	tests := []struct {
		name     string
		tenant   *aigatewayv1alpha1.Tenant
		expected bool
	}{
		{
			name:     "tenant with no conditions",
			tenant:   &aigatewayv1alpha1.Tenant{},
			expected: false,
		},
		{
			name: "tenant with Ready=True",
			tenant: &aigatewayv1alpha1.Tenant{
				Status: aigatewayv1alpha1.TenantStatus{
					Conditions: []metav1.Condition{
						{Type: "Ready", Status: metav1.ConditionTrue},
					},
				},
			},
			expected: true,
		},
		{
			name: "tenant with Ready=False",
			tenant: &aigatewayv1alpha1.Tenant{
				Status: aigatewayv1alpha1.TenantStatus{
					Conditions: []metav1.Condition{
						{Type: "Ready", Status: metav1.ConditionFalse},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTenantReady(tt.tenant)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGatewayAuthenticationCapabilityAndTenantGrants(t *testing.T) {
	tests := []struct {
		name   string
		grant  aigatewayv1alpha1.IdentityType
		auth   []aigatewayv1alpha1.IdentityTypeSpec
		status metav1.ConditionStatus
		reason string
	}{
		{
			name: "extension", grant: "externalApiKey",
			auth:   []aigatewayv1alpha1.IdentityTypeSpec{{Name: "consumers", AuthType: "externalApiKey"}},
			status: metav1.ConditionTrue, reason: aigatewayv1alpha1.ReasonReady,
		},
		{
			name: "case mismatch", grant: "APIKey",
			auth:   []aigatewayv1alpha1.IdentityTypeSpec{{Name: "consumers", AuthType: "apiKey"}},
			status: metav1.ConditionFalse, reason: aigatewayv1alpha1.ReasonInvalid,
		},
		{
			name: "matching case-distinct ID", grant: "APIKey",
			auth:   []aigatewayv1alpha1.IdentityTypeSpec{{Name: "custom", AuthType: "APIKey"}},
			status: metav1.ConditionTrue, reason: aigatewayv1alpha1.ReasonReady,
		},
		{
			name: "multiple entries", grant: "apiKey",
			auth: []aigatewayv1alpha1.IdentityTypeSpec{
				{Name: "one", AuthType: "apiKey"}, {Name: "two", AuthType: "apiKey"},
			},
			status: metav1.ConditionFalse, reason: aigatewayv1alpha1.ReasonInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := aigatewayv1alpha1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			tenant := &aigatewayv1alpha1.Tenant{
				ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: "test", Generation: 2},
				Spec:       aigatewayv1alpha1.TenantSpec{AuthorizedIdentities: []aigatewayv1alpha1.AuthorizedIdentity{{Type: tt.grant}}},
				Status:     aigatewayv1alpha1.TenantStatus{Conditions: []metav1.Condition{{Type: aigatewayv1alpha1.ConditionTypeReady, Status: metav1.ConditionTrue, ObservedGeneration: 2}}},
			}
			gateway := &aigatewayv1alpha1.AIGateway{
				ObjectMeta: metav1.ObjectMeta{Name: "gateway", Namespace: "test", Generation: 4},
				Spec:       aigatewayv1alpha1.AIGatewaySpec{TenantRef: aigatewayv1alpha1.LocalObjectReference{Name: "owner"}, Governance: &aigatewayv1alpha1.GovernanceSpec{Authentication: tt.auth}},
			}
			r := AIGatewayReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tenant).Build()}
			if err := r.validateAndUpdateStatus(context.Background(), gateway); err != nil {
				t.Fatal(err)
			}
			condition := meta.FindStatusCondition(gateway.Status.Conditions, aigatewayv1alpha1.ConditionTypeReady)
			if condition == nil || condition.Status != tt.status || condition.Reason != tt.reason || condition.ObservedGeneration != 4 {
				t.Fatalf("unexpected Ready condition: %+v", condition)
			}
		})
	}
}
