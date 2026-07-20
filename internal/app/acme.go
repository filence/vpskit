package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"vpskit.local/vpskit/internal/fsutil"
)

const (
	defaultACMEServer  = "https://acme-v02.api.letsencrypt.org/directory"
	zeroSSLACMEServer  = "https://acme.zerossl.com/v2/DV90"
	zeroSSLEABEndpoint = "https://api.zerossl.com/acme/eab-credentials-email"
	acmeServerEnvKey   = "VPSKIT_ACME_SERVER"
	acmeEmailEnvKey    = "VPSKIT_ACME_EMAIL"
	acmeEABKIDEnvKey   = "VPSKIT_ACME_EAB_KID"
	acmeEABHMACEnvKey  = "VPSKIT_ACME_EAB_HMAC"
)

type acmeConfig struct {
	Server  string
	Email   string
	EABKID  string
	EABHMAC string
}

type zeroSSLEABResponse struct {
	Success bool   `json:"success"`
	KID     string `json:"eab_kid"`
	HMAC    string `json:"eab_hmac_key"`
}

func readACMEConfig() (acmeConfig, error) {
	config := acmeConfig{Server: defaultACMEServer}
	if content, err := os.ReadFile(acmeEnvPath); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			key, value, found := strings.Cut(line, "=")
			if !found {
				continue
			}
			switch strings.TrimSpace(key) {
			case "ACME_SERVER":
				config.Server = strings.TrimSpace(value)
			case "ACME_EMAIL":
				config.Email = strings.TrimSpace(value)
			case "ACME_EAB_KID":
				config.EABKID = strings.TrimSpace(value)
			case "ACME_EAB_HMAC":
				config.EABHMAC = strings.TrimSpace(value)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return acmeConfig{}, fmt.Errorf("read ACME configuration: %w", err)
	}
	for key, destination := range map[string]*string{
		acmeServerEnvKey:  &config.Server,
		acmeEmailEnvKey:   &config.Email,
		acmeEABKIDEnvKey:  &config.EABKID,
		acmeEABHMACEnvKey: &config.EABHMAC,
	} {
		if value, ok := os.LookupEnv(key); ok {
			*destination = strings.TrimSpace(value)
		}
	}
	if strings.EqualFold(config.Server, "letsencrypt") || config.Server == "" {
		config.Server = defaultACMEServer
	}
	if strings.EqualFold(config.Server, "zerossl") {
		config.Server = zeroSSLACMEServer
	}
	for name, value := range map[string]string{
		"ACME server": config.Server,
		"ACME email":  config.Email,
		"EAB KID":     config.EABKID,
		"EAB HMAC":    config.EABHMAC,
	} {
		if strings.ContainsAny(value, "\r\n\x00") {
			return acmeConfig{}, fmt.Errorf("%s contains invalid control characters", name)
		}
	}
	if (config.EABKID == "" && config.EABHMAC != "") || (config.EABKID != "" && config.EABHMAC == "") {
		return acmeConfig{}, errors.New("ACME EAB KID and HMAC must be provided together")
	}
	if config.Server == zeroSSLACMEServer && config.EABKID == "" && config.Email == "" {
		return acmeConfig{}, errors.New("ZeroSSL ACME requires an email when EAB KID and HMAC are not provided")
	}
	return config, nil
}

func (config acmeConfig) legoRunArguments(domain, legoStatePath string) []string {
	arguments := []string{
		"run",
		"--accept-tos",
		"--email", config.Email,
		"--dns", "cloudflare",
		"--dns.resolvers", "1.1.1.1:53",
		"--domains", domain,
		"--path", legoStatePath,
		"--server", config.Server,
	}
	return arguments
}

func (config acmeConfig) legoEnvironment(environment []string, cloudflareToken string) []string {
	for _, key := range []string{"CF_DNS_API_TOKEN", "LEGO_EAB", "LEGO_EAB_KID", "LEGO_EAB_HMAC"} {
		environment = appendWithoutKey(environment, key)
	}
	environment = append(environment, "CF_DNS_API_TOKEN="+cloudflareToken)
	if config.EABKID != "" {
		environment = append(environment, "LEGO_EAB=true", "LEGO_EAB_KID="+config.EABKID, "LEGO_EAB_HMAC="+config.EABHMAC)
	}
	return environment
}

func populateZeroSSLEAB(config acmeConfig) (acmeConfig, error) {
	if config.Server != zeroSSLACMEServer || config.EABKID != "" {
		return config, nil
	}
	form := url.Values{}
	form.Set("email", config.Email)
	request, err := http.NewRequest(http.MethodPost, zeroSSLEABEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return acmeConfig{}, fmt.Errorf("create ZeroSSL EAB request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return acmeConfig{}, fmt.Errorf("request ZeroSSL EAB: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return acmeConfig{}, fmt.Errorf("ZeroSSL EAB request returned HTTP %d", response.StatusCode)
	}
	var payload zeroSSLEABResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return acmeConfig{}, fmt.Errorf("decode ZeroSSL EAB response: %w", err)
	}
	payload.KID = strings.TrimSpace(payload.KID)
	payload.HMAC = strings.TrimSpace(payload.HMAC)
	if !payload.Success || payload.KID == "" || payload.HMAC == "" {
		return acmeConfig{}, errors.New("ZeroSSL did not return usable EAB credentials")
	}
	if strings.ContainsAny(payload.KID, "\r\n\x00") || strings.ContainsAny(payload.HMAC, "\r\n\x00") {
		return acmeConfig{}, errors.New("ZeroSSL returned invalid EAB credentials")
	}
	config.EABKID = payload.KID
	config.EABHMAC = payload.HMAC
	return config, nil
}

func writeACMEConfig(config acmeConfig) error {
	lines := []string{
		"ACME_SERVER=" + config.Server,
		"ACME_EMAIL=" + config.Email,
	}
	if config.EABKID != "" {
		lines = append(lines, "ACME_EAB_KID="+config.EABKID, "ACME_EAB_HMAC="+config.EABHMAC)
	}
	return fsutil.WriteFileAtomic(acmeEnvPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}
