package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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

func setupBeanBank(ctx context.Context, t *testing.T) (*beanBankContainer, error) {
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

	natPort := nat.Port(port + "/tcp")

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../",
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{string(natPort)},
		Env: map[string]string{
			"PORT":            port,
			"GIN_MODE":        "release",
			"DATABASE_URL":    "sqlite::memory:",
			"JWT_SECRET":      jwtSecret,
			"SESSION_SECRET":  sessionSecret,
			"ADMIN_USERS":     adminUsers,
			"TEST_MODE":       "true",
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

	t.Run("token works for authentication", func(t *testing.T) {
		wallet := getWallet(t, beanBankC.URI, token)
		assert.Equal(t, "tokenuser", wallet["username"].(string))
		assert.Equal(t, 1.0, wallet["bean_amount"].(float64))
	})

	t.Run("token works for transfer", func(t *testing.T) {
		ensureWalletExists(t, beanBankC.URI, "recipient")

		transferBody := strings.NewReader(`{"to_user": "recipient", "amount": 1, "force": false}`)
		req, err := http.NewRequest(http.MethodPost, beanBankC.URI+"/api/v1/transfer", transferBody)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("token works for transactions", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/transactions", nil)
		require.NoError(t, err)
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

	token1 := createToken(t, beanBankC.URI, "multitoken")
	token2 := createToken(t, beanBankC.URI, "multitoken")

	req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/tokens", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token1)

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
		wallet1 := getWallet(t, beanBankC.URI, token1)
		wallet2 := getWallet(t, beanBankC.URI, token2)

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

	token := createToken(t, beanBankC.URI, "deletetest")

	req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/tokens", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

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
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	t.Run("deleted token no longer works", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, beanBankC.URI+"/api/v1/wallet", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
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
