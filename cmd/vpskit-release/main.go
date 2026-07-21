package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vpskit.local/vpskit/internal/fsutil"
	"vpskit.local/vpskit/internal/release"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "vpskit-release:", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit-release <keygen|keyid|trust|manifest|sign|verify|pack|bootstrap>")
	}
	switch arguments[0] {
	case "keygen":
		return keygen(arguments[1:])
	case "keyid":
		return keyid(arguments[1:])
	case "trust":
		return trust(arguments[1:])
	case "manifest":
		return manifest(arguments[1:])
	case "sign":
		return sign(arguments[1:])
	case "verify":
		return verify(arguments[1:])
	case "pack":
		return pack(arguments[1:])
	case "bootstrap":
		return bootstrap(arguments[1:])
	default:
		return errors.New("usage: vpskit-release <keygen|keyid|trust|manifest|sign|verify|pack|bootstrap>")
	}
}

type repeatedStringFlag []string

func (values *repeatedStringFlag) String() string { return fmt.Sprintf("%v", []string(*values)) }

func (values *repeatedStringFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func trust(arguments []string) error {
	flags := flag.NewFlagSet("trust", flag.ContinueOnError)
	var publicPaths repeatedStringFlag
	flags.Var(&publicPaths, "public", "trusted public key path; repeat to include a rotation key")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || len(publicPaths) == 0 {
		return errors.New("usage: vpskit-release trust --public <current-public-key> [--public <next-public-key>]")
	}
	policy := release.TrustPolicy{SchemaVersion: release.TrustPolicySchemaVersion}
	for _, path := range publicPaths {
		value, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		publicKey := strings.TrimSpace(string(value))
		keyID, err := release.PublicKeyID(publicKey)
		if err != nil {
			return err
		}
		policy.Keys = append(policy.Keys, release.TrustedKey{ID: keyID, PublicKey: publicKey})
	}
	encoded, err := release.EncodeTrustPolicy(policy)
	if err != nil {
		return err
	}
	fmt.Println(encoded)
	return nil
}

func bootstrap(arguments []string) error {
	flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	templatePath := flags.String("template", "", "install.sh template path")
	archivePath := flags.String("archive", "", "release archive path")
	output := flags.String("output", "", "generated install.sh path")
	version := flags.String("version", "", "release version")
	repository := flags.String("repository", "", "GitHub owner/repository")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || *templatePath == "" || *archivePath == "" || *output == "" || *version == "" || *repository == "" {
		return errors.New("usage: vpskit-release bootstrap --template <install.sh.in> --archive <archive.tar.gz> --output <install.sh> --version <version> --repository <owner/repository>")
	}
	hash, err := release.GenerateBootstrap(*templatePath, *archivePath, *output, *version, *repository)
	if err != nil {
		return err
	}
	fmt.Printf("bootstrap=%s\narchive_sha256=%s\n", *output, hash)
	return nil
}

func pack(arguments []string) error {
	flags := flag.NewFlagSet("pack", flag.ContinueOnError)
	directory := flags.String("dir", "", "release bundle directory")
	output := flags.String("output", "", "output .tar.gz path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || *directory == "" || *output == "" {
		return errors.New("usage: vpskit-release pack --dir <bundle-directory> --output <archive.tar.gz>")
	}
	if err := release.CreateLinuxArchive(*directory, *output); err != nil {
		return err
	}
	fmt.Printf("archive=%s\n", *output)
	return nil
}

func keyid(arguments []string) error {
	flags := flag.NewFlagSet("keyid", flag.ContinueOnError)
	publicPath := flags.String("public", "", "public key path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *publicPath == "" {
		return errors.New("--public is required")
	}
	publicKey, err := os.ReadFile(*publicPath)
	if err != nil {
		return err
	}
	keyID, err := release.PublicKeyID(string(publicKey))
	if err != nil {
		return err
	}
	fmt.Println(keyID)
	return nil
}

func keygen(arguments []string) error {
	flags := flag.NewFlagSet("keygen", flag.ContinueOnError)
	privatePath := flags.String("private", "", "private key output")
	publicPath := flags.String("public", "", "public key output")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *privatePath == "" || *publicPath == "" {
		return errors.New("--private and --public are required")
	}
	if _, err := os.Stat(*privatePath); err == nil {
		return fmt.Errorf("refusing to overwrite private key: %s", *privatePath)
	}
	publicKey, privateKey, err := release.GenerateKeyPair()
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(*privatePath, []byte(privateKey+"\n"), 0o600); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(*publicPath, []byte(publicKey+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("private=%s\npublic=%s\n", *privatePath, *publicPath)
	return nil
}

func manifest(arguments []string) error {
	flags := flag.NewFlagSet("manifest", flag.ContinueOnError)
	directory := flags.String("dir", "", "bundle directory")
	output := flags.String("output", "", "manifest output path")
	releaseID := flags.String("release-id", "", "release identifier")
	signingKeyID := flags.String("signing-key-id", "", "manifest signing key id")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *directory == "" || *output == "" || *releaseID == "" || *signingKeyID == "" {
		return errors.New("--dir, --output, --release-id, and --signing-key-id are required")
	}
	manifest, err := release.BuildManifest(*releaseID, *signingKeyID, *directory, []release.Asset{
		{ID: "vpskit", Version: *releaseID, File: "vpskit", SourceURL: "vpskit release build"},
		{
			ID:             "sing-box",
			Version:        "1.13.14",
			Channel:        "stable",
			File:           "sing-box",
			SourceURL:      "https://github.com/SagerNet/sing-box/releases/tag/v1.13.14",
			SourceRepo:     "SagerNet/sing-box",
			SourceRef:      "v1.13.14",
			SourceCommit:   "25a600db24f7680ad9806ce5427bd0ab8afe1114",
			StateSchemaMin: 1,
		},
		{
			ID:             "xray",
			Version:        "26.3.27",
			Channel:        "stable",
			File:           "xray",
			SourceURL:      "https://github.com/XTLS/Xray-core/releases/tag/v26.3.27",
			SourceRepo:     "XTLS/Xray-core",
			SourceRef:      "v26.3.27",
			SourceCommit:   "d2758a023cd7f4174a5a5fa4ff66e487d4342ba0",
			StateSchemaMin: 4,
		},
		{
			ID:             "lego",
			Version:        "5.2.2",
			Channel:        "stable",
			File:           "lego",
			SourceURL:      "https://github.com/go-acme/lego/releases/tag/v5.2.2",
			SourceRepo:     "go-acme/lego",
			SourceRef:      "v5.2.2",
			SourceCommit:   "3d5a6695e027d625bd34334d516d77f578d43f11",
			StateSchemaMin: 1,
		},
		{ID: "versions-lock", Version: "1", File: "versions.lock", SourceURL: "vpskit signed upstream asset lock"},
		{ID: "license-vpskit", Version: *releaseID, File: "LICENSE", SourceURL: "vpskit release license"},
		{ID: "notice", Version: *releaseID, File: "NOTICE.md", SourceURL: "vpskit release notices"},
		{ID: "third-party-licenses", Version: *releaseID, File: "THIRD_PARTY_LICENSES.md", SourceURL: "vpskit dependency license inventory"},
		{ID: "sbom-vpskit", Version: *releaseID, File: "vpskit.spdx.json", SourceURL: "Syft SPDX 2.3 build output"},
		{ID: "license-go-qrcode", Version: "v0.0.0-20200617195104-da1b6568686e", File: "licenses/go-qrcode.LICENSE", SourceURL: "https://github.com/skip2/go-qrcode"},
		{ID: "license-go-yaml", Version: "v3.0.4", File: "licenses/go-yaml.LICENSE", SourceURL: "https://github.com/yaml/go-yaml/tree/v3.0.4"},
		{ID: "notice-go-yaml", Version: "v3.0.4", File: "licenses/go-yaml.NOTICE", SourceURL: "https://github.com/yaml/go-yaml/tree/v3.0.4"},
		{ID: "license-lego", Version: "5.2.2", File: "licenses/lego.LICENSE", SourceURL: "https://github.com/go-acme/lego/tree/v5.2.2"},
		{ID: "license-sing-box", Version: "1.13.14", File: "licenses/sing-box.LICENSE", SourceURL: "https://github.com/SagerNet/sing-box/tree/v1.13.14"},
		{ID: "license-xray", Version: "26.3.27", File: "licenses/xray.LICENSE", SourceURL: "https://github.com/XTLS/Xray-core/tree/v26.3.27"},
	})
	if err != nil {
		return err
	}
	bytes, err := release.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(*output, bytes, 0o644); err != nil {
		return err
	}
	fmt.Printf("manifest=%s assets=%d\n", *output, len(manifest.Assets))
	return nil
}

func sign(arguments []string) error {
	flags := flag.NewFlagSet("sign", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "", "manifest path")
	privatePath := flags.String("private", "", "private key path")
	output := flags.String("output", "", "signature output path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	manifestBytes, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	privateKey, err := os.ReadFile(*privatePath)
	if err != nil {
		return err
	}
	signature, err := release.Sign(manifestBytes, string(privateKey))
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(*output, []byte(signature+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("signature=%s\n", *output)
	return nil
}

func verify(arguments []string) error {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	directory := flags.String("dir", "", "bundle directory")
	publicPath := flags.String("public", "", "public key path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	publicKey, err := os.ReadFile(*publicPath)
	if err != nil {
		return err
	}
	manifest, err := release.VerifyDirectory(
		*directory,
		filepath.Join(*directory, "release-manifest.json"),
		filepath.Join(*directory, "release-manifest.sig"),
		string(publicKey),
	)
	if err != nil {
		return err
	}
	fmt.Printf("verified=%s assets=%d\n", manifest.ReleaseID, len(manifest.Assets))
	return nil
}
