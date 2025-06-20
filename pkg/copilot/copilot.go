package copilot

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Copilot struct {
	CopilotToken string
	ExpiresAt    int
}

func findGithubOAuthToken(configPath string) (string, error) {
	filePaths := []string{
		filepath.Join(configPath, "github-copilot", "hosts.json"),
		filepath.Join(configPath, "github-copilot", "apps.json"),
	}

	for _, filePath := range filePaths {
		if _, err := os.Stat(filePath); err == nil {
			content, err := os.ReadFile(filePath)
			if err != nil {
				return "", err
			}

			var userdata map[string]map[string]any
			if err := json.Unmarshal(content, &userdata); err != nil {
				return "", err
			}

			for key, value := range userdata {
				if strings.Contains(key, "github.com") {
					if token, ok := value["oauth_token"].(string); ok {
						return token, nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("no GitHub OAuth token found")
}

func getCopilotInternalToken(bearerToken string) (string, error) {
	url := "https://api.github.com/copilot_internal/v2/token"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "curl")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("non-OK response: %d %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

func getChatCompletion(
	copilotToken string,
	model string,
	messages []map[string]string,
) (string, error) {
	url := "https://api.githubcopilot.com/chat/completions"
	method := "POST"

	payloadData := map[string]any{
		"model":    model,
		"messages": messages,
	}
	payloadBytes, err := json.Marshal(payloadData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}
	payload := strings.NewReader(string(payloadBytes))

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return "", err
	}

	req.Header.Add("Authorization", "Bearer "+copilotToken)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Copilot-Integration-Id", "vscode-chat")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return "", err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	return string(body), nil
}

func unmarshalCopilotToken(tokenString string) (string, int, error) {
	var unmarshaledCopilotToken struct {
		Token     string `json:"token"`
		ExpiresAt int    `json:"expires_at"`
	}
	err := json.Unmarshal([]byte(tokenString), &unmarshaledCopilotToken)
	if err != nil {
		return "", 0, fmt.Errorf("failed to unmarshal Copilot token: %w", err)
	}

	return unmarshaledCopilotToken.Token, unmarshaledCopilotToken.ExpiresAt, nil
}

func (c *Copilot) loadCopilotToken() error {
	if c.CopilotToken != "" && c.ExpiresAt > int(time.Now().Unix()) {
		return nil
	}

	tmpFilePath := filepath.Join(os.TempDir(), "__jiyeollee_copilot.json")
	if _, err := os.Stat(tmpFilePath); err == nil {
		content, err := os.ReadFile(tmpFilePath)
		if err != nil {
			return fmt.Errorf("failed to read temporary file: %w", err)
		}
		copilotToken, expiresAt, err := unmarshalCopilotToken(string(content))
		if err != nil {
			return fmt.Errorf("failed to unmarshal Copilot token from temporary file: %w", err)
		}
		if copilotToken != "" && expiresAt > int(time.Now().Unix()) {
			c.CopilotToken = copilotToken
			c.ExpiresAt = expiresAt
			return nil
		}
	}

	configPath := os.Getenv("XDG_CONFIG_HOME")
	if configPath == "" {
		configPath = filepath.Join(os.Getenv("HOME"), ".config")
	}

	oauthToken, err := findGithubOAuthToken(configPath)
	if err != nil {
		return fmt.Errorf("failed to find GitHub OAuth token: %w", err)
	}

	copilotTokenString, err := getCopilotInternalToken(oauthToken)
	if err != nil {
		return fmt.Errorf("failed to get Copilot internal token: %w", err)
	}

	copilotToken, expiresAt, err := unmarshalCopilotToken(copilotTokenString)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Copilot token: %w", err)
	}
	c.CopilotToken = copilotToken
	c.ExpiresAt = expiresAt

	tmpFile, err := os.Create(tmpFilePath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer tmpFile.Close()
	_, err = tmpFile.WriteString(copilotTokenString)
	if err != nil {
		return fmt.Errorf("failed to write Copilot token to temporary file: %w", err)
	}

	return nil
}

func (c *Copilot) ChatCompletion(model string, messages []map[string]string) (string, error) {
	err := c.loadCopilotToken()
	if err != nil {
		return "", fmt.Errorf("failed to load Copilot token: %w", err)
	}

	response, err := getChatCompletion(c.CopilotToken, model, messages)
	if err != nil {
		return "", fmt.Errorf("failed to get chat completion: %w", err)
	}
	return response, nil
}

func init() {
}
