package billing

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPolarClientAuth(t *testing.T) {
	// Skip if no credentials are configured
	config := LoadConfigOptional()
	if !config.IsConfigured() {
		t.Skip("Skipping test: POLAR_API_KEY, POLAR_WEBHOOK_SECRET, or POLAR_PROJECT_ID not set")
	}

	// Skip if using dummy values from .env.example
	if config.PolarAPIKey == "polar_sk_..." || config.PolarAPIKey == "" {
		t.Skip("Skipping test: POLAR_API_KEY is not configured with real credentials")
	}

	// Create client (use OrgID if ProjectID is not set)
	projectOrOrgID := config.PolarProjectID
	if projectOrOrgID == "" {
		projectOrOrgID = config.PolarOrgID
	}
	client := NewPolarClient(config.PolarAPIKey, projectOrOrgID)

	// Try to list products (this will fail if credentials are invalid)
	products, err := client.ListProducts(context.Background())
	if err != nil {
		t.Fatalf("Failed to authenticate with Polar API: %v", err)
	}

	t.Logf("Successfully authenticated with Polar API - found %d products", len(products))
}

func TestKubernetesSecretLoading(t *testing.T) {
	// Create fake Kubernetes client
	fakeClient := fake.NewSimpleClientset()

	// Create test secret
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "polar-credentials",
			Namespace: "orzbob-system",
		},
		Data: map[string][]byte{
			"api-key":        []byte("polar_sk_test"),
			"webhook-secret": []byte("whsec_test"),
			"project-id":     []byte("proj_test"),
		},
	}

	// Add secret to fake client
	_, err := fakeClient.CoreV1().Secrets("orzbob-system").Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test secret: %v", err)
	}

	// Retrieve secret
	retrieved, err := fakeClient.CoreV1().Secrets("orzbob-system").Get(context.Background(), "polar-credentials", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to retrieve secret: %v", err)
	}

	// Verify data
	if string(retrieved.Data["api-key"]) != "polar_sk_test" {
		t.Errorf("API key mismatch: got %s, want polar_sk_test", string(retrieved.Data["api-key"]))
	}
	if string(retrieved.Data["webhook-secret"]) != "whsec_test" {
		t.Errorf("Webhook secret mismatch: got %s, want whsec_test", string(retrieved.Data["webhook-secret"]))
	}
	if string(retrieved.Data["project-id"]) != "proj_test" {
		t.Errorf("Project ID mismatch: got %s, want proj_test", string(retrieved.Data["project-id"]))
	}
}