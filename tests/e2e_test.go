package tests

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type beanBankContainer struct {
	testcontainers.Container
	URI string
}

var (
	testImageTag   = "bean-bank-test:latest"
	imageBuildOnce sync.Once
	imageBuildErr  error
)

func setupBeanBank(ctx context.Context, t *testing.T) (*beanBankContainer, error) {
	return setupBeanBankWithTestMode(ctx, t, true)
}

func setupBeanBankWithTestMode(ctx context.Context, t *testing.T, testMode bool) (*beanBankContainer, error) {
	imageBuildOnce.Do(func() {
		t.Log("Building Docker image once for all tests...")
		buildReq := testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:       "../",
				Dockerfile:    "Dockerfile",
				Repo:          "bean-bank-test",
				Tag:           "latest",
				PrintBuildLog: false,
				KeepImage:     true,
			},
		}

		_, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: buildReq,
			Started:          false,
		})
		if err != nil {
			imageBuildErr = fmt.Errorf("failed to build image: %w", err)
		}
	})

	if imageBuildErr != nil {
		return nil, imageBuildErr
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "test-secret"
	}

	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		sessionSecret = "test-session-secret"
	}

	adminUsers := os.Getenv("ADMIN_USERS")
	if adminUsers == "" {
		adminUsers = "admin"
	}

	exportSigningKey := os.Getenv("EXPORT_SIGNING_KEY")
	if exportSigningKey == "" {
		exportSigningKey = "test-export-signing-key-for-e2e-tests"
	}

	natPort := nat.Port(port + "/tcp")

	testModeStr := "false"
	if testMode {
		testModeStr = "true"
	}

	req := testcontainers.ContainerRequest{
		Image:        testImageTag,
		ExposedPorts: []string{string(natPort)},
		Env: map[string]string{
			"PORT":               port,
			"GIN_MODE":           "release",
			"DATABASE_URL":       "sqlite::memory:",
			"JWT_SECRET":         jwtSecret,
			"SESSION_SECRET":     sessionSecret,
			"ADMIN_USERS":        adminUsers,
			"TEST_MODE":          testModeStr,
			"EXPORT_SIGNING_KEY": exportSigningKey,
		},
		WaitingFor: wait.ForHTTP("/").
			WithPort(natPort).
			WithStatusCodeMatcher(func(status int) bool {
				return status == 200 || status == 404
			}).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	var beanBankC *beanBankContainer
	if container != nil {
		beanBankC = &beanBankContainer{Container: container}
	}
	if err != nil {
		return beanBankC, err
	}

	host, err := container.Host(ctx)
	if err != nil {
		return beanBankC, err
	}

	mappedPort, err := container.MappedPort(ctx, natPort)
	if err != nil {
		return beanBankC, err
	}

	beanBankC.URI = fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
	return beanBankC, nil
}

func TestE2E_TotalBeans(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	resp, err := http.Get(beanBankC.URI + "/api/v1/total")
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
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	ensureWalletExists(t, beanBankC.URI, "alice")

	reqBody := strings.NewReader(`{"expires_in": "1h"}`)
	req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/tokens", reqBody)
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
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	ensureWalletExists(t, beanBankC.URI, "bob")

	req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/wallet", nil)
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
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	ensureWalletExists(t, beanBankC.URI, "alice")
	ensureWalletExists(t, beanBankC.URI, "bob")

	bobWallet := getWalletTestMode(t, beanBankC.URI, "bob")
	bobInitialBalance := bobWallet["bean_amount"].(float64)

	transferBody := strings.NewReader(`{"to_user": "bob", "amount": 1, "force": false}`)
	req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transfer", transferBody)
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

	bobWalletAfter := getWalletTestMode(t, beanBankC.URI, "bob")
	bobFinalBalance := bobWalletAfter["bean_amount"].(float64)

	assert.Equal(t, bobInitialBalance+1.0, bobFinalBalance, "bob should have received 1 bean")
}

func TestE2E_TransferWithForce(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	ensureWalletExists(t, beanBankC.URI, "charlie")

	transferBody := strings.NewReader(`{"to_user": "newuser", "amount": 1, "force": true}`)
	req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transfer", transferBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Username", "charlie")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	newuserWallet := getWalletTestMode(t, beanBankC.URI, "newuser")
	newuserBalance := newuserWallet["bean_amount"].(float64)

	assert.Equal(t, 1.0, newuserBalance, "newuser should have 1 bean (force-created with 0 + 1 transfer)")
}

func TestE2E_GetTransactions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	ensureWalletExists(t, beanBankC.URI, "dave")
	ensureWalletExists(t, beanBankC.URI, "eve")

	transferBody := strings.NewReader(`{"to_user": "eve", "amount": 1, "force": false}`)
	req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transfer", transferBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Username", "dave")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	req, err = http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/transactions", nil)
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

func TestE2E_TokenAuthentication(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	token := createToken(t, beanBankC.URI, "tokenuser")

	t.Run("token_contains_correct_username", func(t *testing.T) {
		parts := strings.Split(token, ".")
		require.Equal(t, 3, len(parts), "JWT should have 3 parts")

		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		require.NoError(t, err)

		var claims map[string]interface{}
		err = json.Unmarshal(payload, &claims)
		require.NoError(t, err)

		assert.Equal(t, "tokenuser", claims["username"].(string))
	})

	t.Run("token works for authentication", func(t *testing.T) {
		wallet := getWalletTestMode(t, beanBankC.URI, "tokenuser")
		assert.Equal(t, "tokenuser", wallet["username"].(string))
		assert.Equal(t, 1.0, wallet["bean_amount"].(float64))
	})

	t.Run("token works for transfer", func(t *testing.T) {
		ensureWalletExists(t, beanBankC.URI, "recipient")

		transferBody := strings.NewReader(`{"to_user": "recipient", "amount": 1, "force": false}`)
		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transfer", transferBody)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Username", "tokenuser")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("token works for transactions", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/transactions", nil)
		require.NoError(t, err)
		req.Header.Set("X-Test-Username", "tokenuser")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var transactions []map[string]interface{}
		err = json.Unmarshal(body, &transactions)
		require.NoError(t, err)

		assert.Greater(t, len(transactions), 0, "should have at least one transaction")
	})
}

func TestE2E_TokenList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	_ = createToken(t, beanBankC.URI, "multitoken")
	_ = createToken(t, beanBankC.URI, "multitoken")

	req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/tokens", nil)
	require.NoError(t, err)
	req.Header.Set("X-Test-Username", "multitoken")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var tokens []map[string]interface{}
	err = json.Unmarshal(body, &tokens)
	require.NoError(t, err)

	assert.Len(t, tokens, 2, "user should have 2 tokens")

	t.Run("both tokens work independently", func(t *testing.T) {
		wallet1 := getWalletTestMode(t, beanBankC.URI, "multitoken")
		wallet2 := getWalletTestMode(t, beanBankC.URI, "multitoken")

		assert.Equal(t, "multitoken", wallet1["username"].(string))
		assert.Equal(t, "multitoken", wallet2["username"].(string))
	})
}

func TestE2E_TokenDeletion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	_ = createToken(t, beanBankC.URI, "deletetest")

	req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/tokens", nil)
	require.NoError(t, err)
	req.Header.Set("X-Test-Username", "deletetest")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	var tokens []map[string]interface{}
	err = json.Unmarshal(body, &tokens)
	require.NoError(t, err)
	require.Greater(t, len(tokens), 0, "should have at least one token")

	tokenID := tokens[0]["id"].(float64)

	req, err = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/tokens/%d", beanBankC.URI, int(tokenID)), nil)
	require.NoError(t, err)
	req.Header.Set("X-Test-Username", "deletetest")

	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	t.Run("deleted token no longer in list", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/tokens", nil)
		require.NoError(t, err)
		req.Header.Set("X-Test-Username", "deletetest")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var tokens []map[string]interface{}
		err = json.Unmarshal(body, &tokens)
		require.NoError(t, err)

		assert.Equal(t, 0, len(tokens), "token list should be empty after deletion")
	})
}

func TestE2E_InvalidToken(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	t.Run("invalid token returns 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/wallet", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer invalid_token_here")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("missing authorization header returns 401", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/wallet", nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

func TestE2E_TransactionExportAndVerify(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	ensureWalletExists(t, beanBankC.URI, "exporter")
	ensureWalletExists(t, beanBankC.URI, "receiver1")
	ensureWalletExists(t, beanBankC.URI, "receiver2")

	adminReq, err := http.NewRequest(http.MethodPut, beanBankC.URI+"/api/v1/admin/wallet/exporter", strings.NewReader(`{"bean_amount": 100}`))
	require.NoError(t, err)
	adminReq.Header.Set("Content-Type", "application/json")
	adminReq.Header.Set("X-Test-Username", "admin")

	adminResp, err := http.DefaultClient.Do(adminReq)
	require.NoError(t, err)
	adminResp.Body.Close()

	transferBody1 := strings.NewReader(`{"to_user": "receiver1", "amount": 5, "force": false}`)
	req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transfer", transferBody1)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Username", "exporter")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	transferBody2 := strings.NewReader(`{"to_user": "receiver2", "amount": 3, "force": false}`)
	req, err = http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transfer", transferBody2)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Username", "exporter")

	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	t.Run("export_transactions_generates_signed_data", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/transactions/export", nil)
		require.NoError(t, err)
		req.Header.Set("X-Test-Username", "exporter")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var exportData map[string]interface{}
		err = json.Unmarshal(body, &exportData)
		require.NoError(t, err)

		signature, ok := exportData["signature"].(string)
		assert.True(t, ok, "signature should be a string")
		assert.NotEmpty(t, signature, "signature should not be empty")
		assert.Greater(t, len(signature), 32, "signature should be hex-encoded HMAC-SHA256")

		username, ok := exportData["username"].(string)
		assert.True(t, ok)
		assert.Equal(t, "exporter", username)

		transactions, ok := exportData["transactions"].([]interface{})
		assert.True(t, ok)
		assert.GreaterOrEqual(t, len(transactions), 2, "should have at least 2 transactions")
	})

	t.Run("valid_signature_verifies_successfully", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/transactions/export", nil)
		require.NoError(t, err)
		req.Header.Set("X-Test-Username", "exporter")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		req, err = http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transactions/verify", strings.NewReader(string(body)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		resultBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var verifyResult map[string]interface{}
		err = json.Unmarshal(resultBody, &verifyResult)
		require.NoError(t, err)

		valid, ok := verifyResult["valid"].(bool)
		assert.True(t, ok)
		assert.True(t, valid, "valid signature should verify successfully")
	})
}

func TestE2E_TransactionExportTampering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	ensureWalletExists(t, beanBankC.URI, "tampertest")
	ensureWalletExists(t, beanBankC.URI, "tamperreceiver")

	adminReq, err := http.NewRequest(http.MethodPut, beanBankC.URI+"/api/v1/admin/wallet/tampertest", strings.NewReader(`{"bean_amount": 100}`))
	require.NoError(t, err)
	adminReq.Header.Set("Content-Type", "application/json")
	adminReq.Header.Set("X-Test-Username", "admin")

	adminResp, err := http.DefaultClient.Do(adminReq)
	require.NoError(t, err)
	adminResp.Body.Close()

	transferBody := strings.NewReader(`{"to_user": "tamperreceiver", "amount": 10, "force": false}`)
	req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transfer", transferBody)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Username", "tampertest")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	req, err = http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/transactions/export", nil)
	require.NoError(t, err)
	req.Header.Set("X-Test-Username", "tampertest")

	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	var exportData map[string]interface{}
	err = json.Unmarshal(body, &exportData)
	require.NoError(t, err)

	t.Run("valid_export_passes_verification", func(t *testing.T) {
		verifyJSON, err := json.Marshal(exportData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transactions/verify", strings.NewReader(string(verifyJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var verifyResult map[string]interface{}
		err = json.Unmarshal(body, &verifyResult)
		require.NoError(t, err)

		valid, ok := verifyResult["valid"].(bool)
		assert.True(t, ok)
		assert.True(t, valid, "valid export should pass verification")
	})

	t.Run("tampered_data_fails_verification", func(t *testing.T) {
		tamperedData := make(map[string]interface{})
		for k, v := range exportData {
			tamperedData[k] = v
		}
		tamperedData["total_beans"] = 999999.0

		verifyJSON, err := json.Marshal(tamperedData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transactions/verify", strings.NewReader(string(verifyJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var verifyResult map[string]interface{}
		err = json.Unmarshal(body, &verifyResult)
		require.NoError(t, err)

		valid, ok := verifyResult["valid"].(bool)
		assert.True(t, ok)
		assert.False(t, valid, "tampered data should fail verification")
	})

	t.Run("tampered_signature_fails_verification", func(t *testing.T) {
		tamperedData := make(map[string]interface{})
		for k, v := range exportData {
			tamperedData[k] = v
		}
		tamperedData["signature"] = "0000000000000000000000000000000000000000000000000000000000000000"

		verifyJSON, err := json.Marshal(tamperedData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transactions/verify", strings.NewReader(string(verifyJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var verifyResult map[string]interface{}
		err = json.Unmarshal(body, &verifyResult)
		require.NoError(t, err)

		valid, ok := verifyResult["valid"].(bool)
		assert.True(t, ok)
		assert.False(t, valid, "tampered signature should fail verification")
	})

	t.Run("both_data_and_signature_tampered_fails_verification", func(t *testing.T) {
		tamperedData := make(map[string]interface{})
		for k, v := range exportData {
			tamperedData[k] = v
		}
		tamperedData["total_beans"] = 888888.0
		tamperedData["signature"] = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

		verifyJSON, err := json.Marshal(tamperedData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transactions/verify", strings.NewReader(string(verifyJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var verifyResult map[string]interface{}
		err = json.Unmarshal(body, &verifyResult)
		require.NoError(t, err)

		valid, ok := verifyResult["valid"].(bool)
		assert.True(t, ok)
		assert.False(t, valid, "tampered data and signature should fail verification")
	})

	t.Run("invalid_json_data_returns_error", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transactions/verify", strings.NewReader("{invalid json here}"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("missing_signature_returns_error", func(t *testing.T) {
		noSigData := make(map[string]interface{})
		for k, v := range exportData {
			if k != "signature" {
				noSigData[k] = v
			}
		}

		verifyJSON, err := json.Marshal(noSigData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transactions/verify", strings.NewReader(string(verifyJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestE2E_HarvestCompleteWorkflow(t *testing.T) {
	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	defer func() {
		if err := beanBankC.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	}()

	t.Run("complete_harvest_workflow", func(t *testing.T) {
		harvestUser := "harvest_user"
		
		// Step 1: Create a harvest as admin
		harvestData := map[string]interface{}{
			"title":       "Test Harvest Task",
			"description": "Complete this task to earn beans",
			"bean_amount": 25,
		}
		harvestJSON, err := json.Marshal(harvestData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/admin/harvests", strings.NewReader(string(harvestJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Username", "admin")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode, "Failed to create harvest")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var harvestResponse map[string]interface{}
		err = json.Unmarshal(body, &harvestResponse)
		require.NoError(t, err)
		
		harvestID := int(harvestResponse["id"].(float64))
		t.Logf("Created harvest with ID: %d", harvestID)

		// Step 2: Check user's initial balance (should be 1 bean - initial balance)
		req, err = http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/wallet", nil)
		require.NoError(t, err)
		req.Header.Set("X-Test-Username", harvestUser)

		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		require.NoError(t, err)

		var walletBefore map[string]interface{}
		err = json.Unmarshal(body, &walletBefore)
		require.NoError(t, err)

		initialBalance := int(walletBefore["bean_amount"].(float64))
		t.Logf("User initial balance: %d beans", initialBalance)
		assert.Equal(t, 1, initialBalance, "User should start with 1 bean")

		// Step 3: Assign harvest to user as admin
		assignData := map[string]string{
			"username": harvestUser,
		}
		assignJSON, err := json.Marshal(assignData)
		require.NoError(t, err)

		req, err = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/admin/harvests/%d/assign", beanBankC.URI, harvestID), strings.NewReader(string(assignJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Username", "admin")

		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to assign user to harvest")
		t.Logf("Assigned harvest to user: %s", harvestUser)

		// Step 4: Check user's balance again (should still be 1 bean - not completed yet)
		req, err = http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/wallet", nil)
		require.NoError(t, err)
		req.Header.Set("X-Test-Username", harvestUser)

		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		require.NoError(t, err)

		var walletAfterAssign map[string]interface{}
		err = json.Unmarshal(body, &walletAfterAssign)
		require.NoError(t, err)

		balanceAfterAssign := int(walletAfterAssign["bean_amount"].(float64))
		t.Logf("User balance after assignment: %d beans", balanceAfterAssign)
		assert.Equal(t, initialBalance, balanceAfterAssign, "Balance should not change after assignment")

		// Step 5: Admin marks harvest as complete
		req, err = http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/admin/harvests/%d/complete", beanBankC.URI, harvestID), nil)
		require.NoError(t, err)
		req.Header.Set("X-Test-Username", "admin")

		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode, "Failed to complete harvest")
		t.Logf("Marked harvest as complete")

		// Step 6: Check user's balance after completion (should have initial + reward beans)
		req, err = http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/wallet", nil)
		require.NoError(t, err)
		req.Header.Set("X-Test-Username", harvestUser)

		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		require.NoError(t, err)

		var walletFinal map[string]interface{}
		err = json.Unmarshal(body, &walletFinal)
		require.NoError(t, err)

		finalBalance := int(walletFinal["bean_amount"].(float64))
		expectedBalance := initialBalance + 25
		t.Logf("User final balance: %d beans (expected: %d)", finalBalance, expectedBalance)
		assert.Equal(t, expectedBalance, finalBalance, "User should have received 25 beans after harvest completion")

		// Step 7: Verify harvest is marked as completed
		req, err = http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/harvests", nil)
		require.NoError(t, err)

		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		require.NoError(t, err)

		var harvestsResponse map[string]interface{}
		err = json.Unmarshal(body, &harvestsResponse)
		require.NoError(t, err)

		harvestsList := harvestsResponse["harvests"].([]interface{})
		harvests := make([]map[string]interface{}, len(harvestsList))
		for i, h := range harvestsList {
			harvests[i] = h.(map[string]interface{})
		}
		var completedHarvest map[string]interface{}
		for _, h := range harvests {
			if int(h["id"].(float64)) == harvestID {
				completedHarvest = h
				break
			}
		}

		require.NotNil(t, completedHarvest, "Harvest should be in the list")
		assert.True(t, completedHarvest["completed"].(bool), "Harvest should be marked as completed")
		assert.Equal(t, harvestUser, completedHarvest["assigned_user"].(string), "Harvest should be assigned to the correct user")
		t.Logf("✓ Harvest workflow completed successfully")
	})
}

func TestE2E_GiftLinkCreateAndList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	ensureWalletExists(t, beanBankC.URI, "giftcreator")

	adminReq, err := http.NewRequest(http.MethodPut, beanBankC.URI+"/api/v1/admin/wallet/giftcreator", strings.NewReader(`{"bean_amount": 100}`))
	require.NoError(t, err)
	adminReq.Header.Set("Content-Type", "application/json")
	adminReq.Header.Set("X-Test-Username", "admin")

	adminResp, err := http.DefaultClient.Do(adminReq)
	require.NoError(t, err)
	adminResp.Body.Close()

	t.Run("create_gift_link", func(t *testing.T) {
		giftData := map[string]interface{}{
			"amount":     50,
			"message":    "Happy testing! 🎉",
			"expires_in": "24h",
		}
		giftJSON, err := json.Marshal(giftData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/giftlinks", strings.NewReader(string(giftJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Username", "giftcreator")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		assert.NotEmpty(t, result["code"], "gift link should have a code")
		assert.Equal(t, float64(50), result["amount"])
		assert.Equal(t, "Happy testing! 🎉", result["message"])
		assert.True(t, result["active"].(bool))
		t.Logf("Created gift link with code: %s", result["code"])
	})

	t.Run("list_gift_links", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/giftlinks", nil)
		require.NoError(t, err)
		req.Header.Set("X-Test-Username", "giftcreator")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var giftLinks []map[string]interface{}
		err = json.Unmarshal(body, &giftLinks)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(giftLinks), 1, "should have at least 1 gift link")
		assert.Equal(t, "giftcreator", giftLinks[0]["from_username"])
		t.Logf("Found %d gift links for user", len(giftLinks))
	})
}

func TestE2E_GiftLinkRedeem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	ensureWalletExists(t, beanBankC.URI, "giftsender")
	ensureWalletExists(t, beanBankC.URI, "giftreceiver")

	adminReq, err := http.NewRequest(http.MethodPut, beanBankC.URI+"/api/v1/admin/wallet/giftsender", strings.NewReader(`{"bean_amount": 100}`))
	require.NoError(t, err)
	adminReq.Header.Set("Content-Type", "application/json")
	adminReq.Header.Set("X-Test-Username", "admin")

	adminResp, err := http.DefaultClient.Do(adminReq)
	require.NoError(t, err)
	adminResp.Body.Close()

	giftData := map[string]interface{}{
		"amount":  30,
		"message": "Gift for you!",
	}
	giftJSON, err := json.Marshal(giftData)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/giftlinks", strings.NewReader(string(giftJSON)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Username", "giftsender")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	var giftLink map[string]interface{}
	err = json.Unmarshal(body, &giftLink)
	require.NoError(t, err)

	giftCode := giftLink["code"].(string)
	t.Logf("Created gift link with code: %s", giftCode)

	senderWallet := getWalletTestMode(t, beanBankC.URI, "giftsender")
	senderBalanceBefore := int(senderWallet["bean_amount"].(float64))

	receiverWallet := getWalletTestMode(t, beanBankC.URI, "giftreceiver")
	receiverBalanceBefore := int(receiverWallet["bean_amount"].(float64))

	t.Run("redeem_gift_link", func(t *testing.T) {
		redeemData := map[string]string{
			"code": giftCode,
		}
		redeemJSON, err := json.Marshal(redeemData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/gift/redeem", strings.NewReader(string(redeemJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Username", "giftreceiver")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Logf("Redeem failed with status %d: %s", resp.StatusCode, string(body))
		}

		require.Equal(t, http.StatusOK, resp.StatusCode)

		senderWalletAfter := getWalletTestMode(t, beanBankC.URI, "giftsender")
		senderBalanceAfter := int(senderWalletAfter["bean_amount"].(float64))

		receiverWalletAfter := getWalletTestMode(t, beanBankC.URI, "giftreceiver")
		receiverBalanceAfter := int(receiverWalletAfter["bean_amount"].(float64))

		assert.Equal(t, senderBalanceBefore, senderBalanceAfter, "sender balance should not change on redemption")
		assert.Equal(t, receiverBalanceBefore+30, receiverBalanceAfter, "receiver should have 30 more beans")
		t.Logf("✓ Gift link redeemed successfully: sender=%d, receiver=%d", senderBalanceAfter, receiverBalanceAfter)
	})

	t.Run("cannot_redeem_twice", func(t *testing.T) {
		redeemData := map[string]string{
			"code": giftCode,
		}
		redeemJSON, err := json.Marshal(redeemData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/gift/redeem", strings.NewReader(string(redeemJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Username", "giftreceiver")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "should not be able to redeem already redeemed gift link")
	})
}

func TestE2E_GiftLinkDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	ensureWalletExists(t, beanBankC.URI, "giftdeleter")

	adminReq, err := http.NewRequest(http.MethodPut, beanBankC.URI+"/api/v1/admin/wallet/giftdeleter", strings.NewReader(`{"bean_amount": 100}`))
	require.NoError(t, err)
	adminReq.Header.Set("Content-Type", "application/json")
	adminReq.Header.Set("X-Test-Username", "admin")

	adminResp, err := http.DefaultClient.Do(adminReq)
	require.NoError(t, err)
	adminResp.Body.Close()

	giftData := map[string]interface{}{
		"amount":  40,
		"message": "To be deleted",
	}
	giftJSON, err := json.Marshal(giftData)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/giftlinks", strings.NewReader(string(giftJSON)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Username", "giftdeleter")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	var giftLink map[string]interface{}
	err = json.Unmarshal(body, &giftLink)
	require.NoError(t, err)

	giftID := int(giftLink["id"].(float64))
	t.Logf("Created gift link with ID: %d", giftID)

	walletBefore := getWalletTestMode(t, beanBankC.URI, "giftdeleter")
	balanceBefore := int(walletBefore["bean_amount"].(float64))

	t.Run("delete_gift_link_refunds_beans", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/giftlinks/%d", beanBankC.URI, giftID), nil)
		require.NoError(t, err)
		req.Header.Set("X-Test-Username", "giftdeleter")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		walletAfter := getWalletTestMode(t, beanBankC.URI, "giftdeleter")
		balanceAfter := int(walletAfter["bean_amount"].(float64))

		assert.Equal(t, balanceBefore+40, balanceAfter, "should refund 40 beans after deletion")
		t.Logf("✓ Gift link deleted and beans refunded: balance=%d", balanceAfter)
	})
}

func TestE2E_GiftLinkEdgeCases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test")
	}

	ctx := context.Background()
	beanBankC, err := setupBeanBank(ctx, t)
	require.NoError(t, err)
	testcontainers.CleanupContainer(t, beanBankC)

	ensureWalletExists(t, beanBankC.URI, "edgecaseuser")

	adminReq, err := http.NewRequest(http.MethodPut, beanBankC.URI+"/api/v1/admin/wallet/edgecaseuser", strings.NewReader(`{"bean_amount": 100}`))
	require.NoError(t, err)
	adminReq.Header.Set("Content-Type", "application/json")
	adminReq.Header.Set("X-Test-Username", "admin")

	adminResp, err := http.DefaultClient.Do(adminReq)
	require.NoError(t, err)
	adminResp.Body.Close()

	t.Run("cannot_create_gift_with_insufficient_balance", func(t *testing.T) {
		giftData := map[string]interface{}{
			"amount":  200,
			"message": "Too much!",
		}
		giftJSON, err := json.Marshal(giftData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/giftlinks", strings.NewReader(string(giftJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Username", "edgecaseuser")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "should not allow gift creation with insufficient balance")
	})

	t.Run("cannot_redeem_own_gift_link", func(t *testing.T) {
		giftData := map[string]interface{}{
			"amount":  10,
			"message": "For myself?",
		}
		giftJSON, err := json.Marshal(giftData)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/giftlinks", strings.NewReader(string(giftJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Username", "edgecaseuser")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		var giftLink map[string]interface{}
		err = json.Unmarshal(body, &giftLink)
		require.NoError(t, err)

		giftCode := giftLink["code"].(string)

		redeemData := map[string]string{
			"code": giftCode,
		}
		redeemJSON, err := json.Marshal(redeemData)
		require.NoError(t, err)

		req, err = http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/gift/redeem", strings.NewReader(string(redeemJSON)))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Username", "edgecaseuser")

		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "should not allow user to redeem their own gift link")
	})

	t.Run("cannot_get_nonexistent_gift_info", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/gift/nonexistentcode", nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "should return 404 for nonexistent gift link")
	})
}
