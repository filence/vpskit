package model

import "time"

const SchemaVersion = 5

type State struct {
	SchemaVersion       int            `json:"schema_version"`
	VPSKitVersion       string         `json:"vpskit_version"`
	TransactionID       string         `json:"transaction_id"`
	InstalledAt         time.Time      `json:"installed_at"`
	Profile             string         `json:"profile"`
	ConnectHost         string         `json:"connect_host"`
	Domain              string         `json:"domain"`
	RealityServerName   string         `json:"reality_server_name"`
	Core                CoreState      `json:"core"`
	RealityCore         CoreState      `json:"reality_core"`
	Reality             RealityState   `json:"reality"`
	Hysteria2           Hysteria2State `json:"hysteria2"`
	Firewall            FirewallState  `json:"firewall"`
	ConfigSHA256        string         `json:"config_sha256"`
	RealityConfigSHA256 string         `json:"reality_config_sha256"`
	ConfigRevision      int            `json:"config_revision"`
	Exports             []ExportState  `json:"exports"`
}

type CoreState struct {
	ID           string `json:"id"`
	Version      string `json:"version"`
	Channel      string `json:"channel"`
	Path         string `json:"path"`
	SHA256       string `json:"sha256"`
	SourceURL    string `json:"source_url,omitempty"`
	SourceRef    string `json:"source_ref,omitempty"`
	SourceCommit string `json:"source_commit,omitempty"`
}

type RealityState struct {
	Enabled       bool   `json:"enabled"`
	ID            string `json:"id"`
	ListenPort    int    `json:"listen_port"`
	UUIDRef       string `json:"uuid_ref"`
	PrivateKeyRef string `json:"private_key_ref"`
	PublicKey     string `json:"public_key"`
	ShortID       string `json:"short_id"`
}

type Hysteria2State struct {
	Enabled              bool   `json:"enabled"`
	ID                   string `json:"id"`
	ListenPort           int    `json:"listen_port"`
	PasswordRef          string `json:"password_ref"`
	CertificatePath      string `json:"certificate_path"`
	KeyPath              string `json:"key_path"`
	CertificateDNS       string `json:"certificate_dns"`
	CertificateAuthority string `json:"certificate_authority,omitempty"`
}

type FirewallState struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
}

type ExportState struct {
	Format string `json:"format"`
	Path   string `json:"path"`
}

type Secrets struct {
	SchemaVersion     int    `json:"schema_version"`
	RealityUUID       string `json:"reality_uuid"`
	RealityPrivateKey string `json:"reality_private_key"`
	Hysteria2Password string `json:"hysteria2_password"`
}

type RuntimeValues struct {
	RealityEnabled    bool
	Hysteria2Enabled  bool
	ConnectHost       string
	Domain            string
	RealityServerName string
	TCPPort           int
	UDPPort           int
	RealityUUID       string
	RealityPrivateKey string
	RealityPublicKey  string
	RealityShortID    string
	Hysteria2Password string
	CertificatePath   string
	KeyPath           string
}
