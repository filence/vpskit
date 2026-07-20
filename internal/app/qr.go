package app

import (
	"errors"
	"fmt"
	"os"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

func runQRExport() error {
	content, err := os.ReadFile(exportRoot + "/share-links.txt")
	if err != nil {
		return fmt.Errorf("read share links for QR export: %w", err)
	}
	output, err := renderTerminalQRCodes(string(content))
	if err != nil {
		return err
	}
	_, err = fmt.Print(output)
	return err
}

func renderTerminalQRCodes(content string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var output strings.Builder
	count := 0
	for _, raw := range lines {
		link := strings.TrimSpace(raw)
		if link == "" {
			continue
		}
		label := "UNKNOWN"
		switch {
		case strings.HasPrefix(link, "vless://"):
			label = "REALITY"
		case strings.HasPrefix(link, "hysteria2://"):
			label = "HYSTERIA2"
		default:
			return "", errors.New("share link file contains an unsupported scheme")
		}
		code, err := qrcode.New(link, qrcode.Medium)
		if err != nil {
			return "", fmt.Errorf("encode %s share link as QR: %w", label, err)
		}
		if count > 0 {
			output.WriteByte('\n')
		}
		output.WriteString("VPSKit ")
		output.WriteString(label)
		output.WriteString(" QR\n")
		output.WriteString(code.ToSmallString(false))
		count++
	}
	if count == 0 {
		return "", errors.New("share link file contains no active links")
	}
	return output.String(), nil
}
