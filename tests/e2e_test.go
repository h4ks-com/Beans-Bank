package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type beapinContainer struct {
	testcontainers.Container
	URI string
}

func setupBeapin(ctx context.Context, t *testing.T) (*beapinContainer, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../",
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			"PORT":            "8080",
			"GIN_MODE":        "release",
			"DATABASE_URL":    "sqlite::memory:",
			"JWT_SECRET":      "test-secret",
			"SESSION_SECRET":  "test-session-secret",
			"ADMIN_USERS":     "admin",
			"TEST_MODE":       "true",
		},
		WaitingFor: wait.ForHTTP("/").
			WithPort("8080/tcp").
			WithStatusCodeMatcher(func(status int) bool {
				return status == 200 || status == 404
			}).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	var beapinC *beapinContainer
	if container != nil {
		beapinC = &beapinContainer{Container: container}
	}
	if err != nil {
		return beapinC, err
	}

	host, err := container.Host(ctx)
	if err != nil {
		return beapinC, err
	}

	mappedPort, err := container.MappedPort(ctx, "8080")
	if err != nil {
		return beapinC, err
	}

	beapinC.URI = fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
	return beapinC, nil
}

func TestE2E_TotalBeans(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beapinC, err := setupBeapin(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beapinC)

	resp, err := http.Get(beapinC.URI + "/api/v1/total")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	totalBeans, ok := result["total_beans"].(float64)
	assert.True(t, ok, "total_beans should be a number")
	assert.GreaterOrEqual(t, totalBeans, 0.0, "total_beans should be >= 0")
}

func TestE2E_CreateToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beapinC, err := setupBeapin(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beapinC)

	ensureWalletExists(t, beapinC.URI, "alice")

	reqBody := strings.NewReader(`{"expires_in": "1h"}`)
	req, err := http.NewRequest(http.MethodPost, beapinC.URI+"/api/v1/tokens", reqBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Username", "alice")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode != http.StatusCreated {
		t.Logf("Response body: %s", string(body))
	}
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	token, ok := result["token"].(string)
	assert.True(t, ok, "token should be a string")
	assert.NotEmpty(t, token, "token should not be empty")
}

func TestE2E_GetWallet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beapinC, err := setupBeapin(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beapinC)

	ensureWalletExists(t, beapinC.URI, "bob")

	req, err := http.NewRequest(http.MethodGet, beapinC.URI+"/api/v1/wallet", nil)
	require.NoError(t, err)
	req.Header.Set("X-Test-Username", "bob")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	username, ok := result["username"].(string)
	assert.True(t, ok)
	assert.Equal(t, "bob", username)

	beanAmount, ok := result["bean_amount"].(float64)
	assert.True(t, ok)
	assert.Equal(t, 1.0, beanAmount, "new user should have 1 bean")
}

func TestE2E_Transfer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beapinC, err := setupBeapin(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beapinC)

	ensureWalletExists(t, beapinC.URI, "alice")
	ensureWalletExists(t, beapinC.URI, "bob")

	bobWallet := getWalletTestMode(t, beapinC.URI, "bob")
	bobInitialBalance := bobWallet["bean_amount"].(float64)

	transferBody := strings.NewReader(`{"to_user": "bob", "amount": 1, "force": false}`)
	req, err := http.NewRequest(http.MethodPost, beapinC.URI+"/api/v1/transfer", transferBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Username", "alice")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Logf("Transfer failed: status=%d, body=%s", resp.StatusCode, string(body))
	}
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	bobWalletAfter := getWalletTestMode(t, beapinC.URI, "bob")
	bobFinalBalance := bobWalletAfter["bean_amount"].(float64)

	assert.Equal(t, bobInitialBalance+1.0, bobFinalBalance, "bob should have received 1 bean")
}

func TestE2E_TransferWithForce(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beapinC, err := setupBeapin(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beapinC)

	ensureWalletExists(t, beapinC.URI, "charlie")

	transferBody := strings.NewReader(`{"to_user": "newuser", "amount": 1, "force": true}`)
	req, err := http.NewRequest(http.MethodPost, beapinC.URI+"/api/v1/transfer", transferBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Username", "charlie")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	newuserWallet := getWalletTestMode(t, beapinC.URI, "newuser")
	newuserBalance := newuserWallet["bean_amount"].(float64)

	assert.Equal(t, 1.0, newuserBalance, "newuser should have 1 bean (force-created with 0 + 1 transfer)")
}

func TestE2E_GetTransactions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beapinC, err := setupBeapin(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beapinC)

	ensureWalletExists(t, beapinC.URI, "dave")
	ensureWalletExists(t, beapinC.URI, "eve")

	transferBody := strings.NewReader(`{"to_user": "eve", "amount": 1, "force": false}`)
	req, err := http.NewRequest(http.MethodPost, beapinC.URI+"/api/v1/transfer", transferBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Username", "dave")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	req, err = http.NewRequest(http.MethodGet, beapinC.URI+"/api/v1/transactions", nil)
	require.NoError(t, err)
	req.Header.Set("X-Test-Username", "dave")

	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var transactions []map[string]interface{}
	err = json.Unmarshal(body, &transactions)
	require.NoError(t, err)

	assert.Greater(t, len(transactions), 0, "should have at least one transaction")

	lastTx := transactions[0]
	assert.Equal(t, "dave", lastTx["from_user"].(string))
	assert.Equal(t, "eve", lastTx["to_user"].(string))
	assert.Equal(t, 1.0, lastTx["amount"].(float64))
}

func createToken(t *testing.T, baseURL, username string) string {
	ensureWalletExists(t, baseURL, username)

	reqBody := strings.NewReader(`{"expires_in": "1h"}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/tokens", reqBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Username", username)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	if resp.StatusCode != http.StatusCreated {
		t.Logf("Create token failed for %s: %s", username, string(body))
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	token, ok := result["token"].(string)
	require.True(t, ok, "token should be a string")
	require.NotEmpty(t, token, "token should not be empty")

	return token
}

func ensureWalletExists(t *testing.T, baseURL, username string) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/wallet", nil)
	require.NoError(t, err)
	req.Header.Set("X-Test-Username", username)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("ensureWalletExists for %s failed: status=%d, body=%s", username, resp.StatusCode, string(body))
	}
	require.Equal(t, http.StatusOK, resp.StatusCode, "wallet should be created/retrieved")
}

func getWallet(t *testing.T, baseURL, token string) map[string]interface{} {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/wallet", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	return result
}

func getWalletTestMode(t *testing.T, baseURL, username string) map[string]interface{} {
	req, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/wallet", nil)
	require.NoError(t, err)
	req.Header.Set("X-Test-Username", username)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)

	return result
}
