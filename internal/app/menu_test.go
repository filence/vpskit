package app

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestMenuExitDoesNotExecuteCommand(t *testing.T) {
	var output bytes.Buffer
	executed := false
	err := runMenuWithIO(strings.NewReader("0\n"), &output, "v-test", "key", func([]string, string, string) error {
		executed = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if executed {
		t.Fatal("exit selection executed a command")
	}
	if !strings.Contains(output.String(), "已退出VPSKit管理菜单") {
		t.Fatalf("missing exit confirmation: %s", output.String())
	}
}

func TestMenuCustomRealityScanBuildsExpectedCommand(t *testing.T) {
	var output bytes.Buffer
	var captured []string
	err := runMenuWithIO(strings.NewReader("5\nwww.amazon.com,www.cloudflare.com\n0\n"), &output, "v-test", "key", func(arguments []string, _, _ string) error {
		captured = append([]string(nil), arguments...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"reality", "scan", "--targets", "www.amazon.com,www.cloudflare.com"}
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("command = %#v, want %#v", captured, want)
	}
}

func TestMenuRealityChangeRequiresExactConfirmation(t *testing.T) {
	var output bytes.Buffer
	executed := false
	err := runMenuWithIO(strings.NewReader("6\nwww.amazon.com\napply\n0\n"), &output, "v-test", "key", func([]string, string, string) error {
		executed = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if executed {
		t.Fatal("unconfirmed Reality target change executed a command")
	}
	if !strings.Contains(output.String(), "操作已取消") {
		t.Fatalf("missing cancellation message: %s", output.String())
	}
}

func TestMenuRealityChangeBuildsExpectedCommand(t *testing.T) {
	var output bytes.Buffer
	var captured []string
	err := runMenuWithIO(strings.NewReader("6\nWWW.AMAZON.COM\nAPPLY\n0\n"), &output, "v-test", "key", func(arguments []string, _, _ string) error {
		captured = append([]string(nil), arguments...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"instance", "modify", "reality", "--reality-server-name", "www.amazon.com"}
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("command = %#v, want %#v", captured, want)
	}
}

func TestMenuNodeNameChangeBuildsExpectedCommand(t *testing.T) {
	var output bytes.Buffer
	var captured []string
	err := runMenuWithIO(strings.NewReader("12\nPersonal-JP-01\nAPPLY\n0\n"), &output, "v-test", "key", func(arguments []string, _, _ string) error {
		captured = append([]string(nil), arguments...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"node", "modify", "--display-name", "Personal-JP-01"}
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("command = %#v, want %#v", captured, want)
	}
}
