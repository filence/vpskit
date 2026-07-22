package app

import (
	"strings"
	"testing"
)

func TestParseHysteria2UDPSocketBuffers(t *testing.T) {
	output := "UNCONN 0 0 *:443 *:*\n\t skmem:(r0,rb2097152,t0,tb2097152,f4096,w0,o0,bl0,d0)\nUNCONN 0 0 *:53 *:*\n\t skmem:(r0,rb212992,t0,tb212992,f0,w0,o0,bl0,d0)\n"
	buffers, err := parseHysteria2UDPSocketBuffers(output, 443)
	if err != nil {
		t.Fatal(err)
	}
	if buffers.ReceiveBytes != hysteria2UDPBufferBytes2MiB || buffers.SendBytes != hysteria2UDPBufferBytes2MiB {
		t.Fatalf("unexpected buffers: %#v", buffers)
	}
}

func TestParseHysteria2UDPSocketBuffersRejectsInvalidOutput(t *testing.T) {
	for _, output := range []string{
		"",
		"UNCONN 0 0 *:443 *:*\n",
		"UNCONN 0 0 *:443 *:*\n skmem:(r0,rb0,t0,tb2097152)\n",
		"UNCONN 0 0 *:53 *:*\n skmem:(r0,rb212992,t0,tb212992)\n",
	} {
		if _, err := parseHysteria2UDPSocketBuffers(output, 443); err == nil {
			t.Fatalf("expected parse failure for %q", output)
		}
	}
}

func TestRenderHysteria2UDPBufferSysctl(t *testing.T) {
	values, err := hysteria2UDPBufferProfile(hysteria2UDPBufferProfileConservative2MiB)
	if err != nil {
		t.Fatal(err)
	}
	contents := renderHysteria2UDPBufferSysctl(values)
	for _, expected := range []string{
		"# Managed by VPSKit.",
		"net.core.rmem_default = 2097152",
		"net.core.wmem_default = 2097152",
		"net.core.rmem_max = 2097152",
		"net.core.wmem_max = 2097152",
	} {
		if !strings.Contains(contents, expected) {
			t.Fatalf("rendered sysctl missing %q: %s", expected, contents)
		}
	}
}

func TestHysteria2UDPBufferProfileRejectsUnsupportedValue(t *testing.T) {
	if _, err := hysteria2UDPBufferProfile("4mib"); err == nil {
		t.Fatal("expected unsupported profile error")
	}
}
