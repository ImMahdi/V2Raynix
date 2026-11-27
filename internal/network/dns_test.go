package network

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseDNSServers(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "valid comma-separated with spaces",
			raw:  " 1.1.1.1 ,  8.8.8.8 ",
			want: []string{"1.1.1.1", "8.8.8.8"},
		},
		{
			name: "filters out invalid IPs",
			raw:  "1.1.1.1, not-an-ip, 9.9.9.9, 999.999.999.999",
			want: []string{"1.1.1.1", "9.9.9.9"},
		},
		{
			name: "empty input falls back to defaults",
			raw:  "",
			want: []string{"1.1.1.1", "8.8.8.8"},
		},
		{
			name: "whitespace only falls back to defaults",
			raw:  "   ,  , ",
			want: []string{"1.1.1.1", "8.8.8.8"},
		},
		{
			name: "all invalid falls back to defaults",
			raw:  "foo, bar, 1234",
			want: []string{"1.1.1.1", "8.8.8.8"},
		},
		{
			name: "supports IPv6",
			raw:  "2606:4700:4700::1111, 1.1.1.1",
			want: []string{"2606:4700:4700::1111", "1.1.1.1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseDNSServers(tc.raw)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseDNSServers(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestBuildResolvectlApplyCommands(t *testing.T) {
	argsList := BuildResolvectlApplyArgs("tun0", []string{"1.1.1.1", "8.8.8.8"})
	var got []string
	for _, args := range argsList {
		got = append(got, "resolvectl "+strings.Join(args, " "))
	}

	want := []string{
		"resolvectl dns tun0 1.1.1.1 8.8.8.8",
		"resolvectl domain tun0 ~.",
		"resolvectl default-route tun0 yes",
		"resolvectl flush-caches",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildResolvectlApplyArgs() =\n%v\nwant:\n%v", got, want)
	}
}

func TestBuildResolvectlRevertCommands(t *testing.T) {
	argsList := BuildResolvectlRevertArgs("tun0")
	var got []string
	for _, args := range argsList {
		got = append(got, "resolvectl "+strings.Join(args, " "))
	}

	want := []string{
		"resolvectl revert tun0",
		"resolvectl flush-caches",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildResolvectlRevertArgs() =\n%v\nwant:\n%v", got, want)
	}
}

func TestResolvConfFallbackApplyAndRevert(t *testing.T) {
	tmpDir := t.TempDir()
	resolvPath := filepath.Join(tmpDir, "etc", "resolv.conf")
	backupPath := filepath.Join(tmpDir, "etc", "v2raynix", "resolv.conf.bak")

	if err := os.MkdirAll(filepath.Dir(resolvPath), 0o755); err != nil {
		t.Fatalf("mkdir resolv dir: %v", err)
	}
	originalContent := "# original resolv.conf\nnameserver 127.0.0.53\noptions edns0\n"
	if err := os.WriteFile(resolvPath, []byte(originalContent), 0o644); err != nil {
		t.Fatalf("write original resolv.conf: %v", err)
	}

	if err := applyResolvConfFallback(resolvPath, backupPath, []string{"1.1.1.1", "8.8.8.8"}); err != nil {
		t.Fatalf("applyResolvConfFallback() error = %v", err)
	}

	bakBytes, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup file: %v", err)
	}
	if string(bakBytes) != originalContent {
		t.Errorf("backup content = %q, want %q", string(bakBytes), originalContent)
	}

	gotBytes, err := os.ReadFile(resolvPath)
	if err != nil {
		t.Fatalf("read updated resolv.conf: %v", err)
	}
	wantApplied := "nameserver 1.1.1.1\nnameserver 8.8.8.8\n"
	if string(gotBytes) != wantApplied {
		t.Errorf("applied resolv.conf = %q, want %q", string(gotBytes), wantApplied)
	}

	if err := revertResolvConfFallback(resolvPath, backupPath); err != nil {
		t.Fatalf("revertResolvConfFallback() error = %v", err)
	}

	restoredBytes, err := os.ReadFile(resolvPath)
	if err != nil {
		t.Fatalf("read restored resolv.conf: %v", err)
	}
	if string(restoredBytes) != originalContent {
		t.Errorf("restored resolv.conf = %q, want %q", string(restoredBytes), originalContent)
	}

	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Errorf("expected backup file %s to be removed, stat err = %v", backupPath, err)
	}
}
