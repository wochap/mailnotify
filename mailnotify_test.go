package main

import (
	"os"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type fakeFileInfo struct {
	name  string
	isDir bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.isDir }
func (f fakeFileInfo) Sys() any           { return nil }

func TestShouldWatch(t *testing.T) {
	tests := []struct {
		path string
		info fakeFileInfo
		want bool
	}{
		// mbsync layout
		{"/mail/Personal/INBOX/new", fakeFileInfo{"new", true}, true},
		{"/mail/BOC/INBOX/new", fakeFileInfo{"new", true}, true},
		{"/mail/Personal/[Gmail]/All Mail/new", fakeFileInfo{"new", true}, false},
		{"/mail/Personal/[Gmail]/Trash/new", fakeFileInfo{"new", true}, false},
		{"/mail/Personal/Trash/new", fakeFileInfo{"new", true}, false},
		{"/mail/Personal/INBOX/cur", fakeFileInfo{"cur", true}, false},
		{"/mail/Personal/INBOX/new/file", fakeFileInfo{"file", false}, false},

		// lieer layout
		{"/mail/personal/mail/new", fakeFileInfo{"new", true}, true},
		{"/mail/se/mail/new", fakeFileInfo{"new", true}, true},
		{"/mail/personal/mail/cur", fakeFileInfo{"cur", true}, true},
		{"/mail/personal/mail/tmp", fakeFileInfo{"tmp", true}, true},
		{"/mail/tmp", fakeFileInfo{"tmp", true}, true},
	}

	for _, tt := range tests {
		if got := shouldWatch(tt.path, tt.info); got != tt.want {
			t.Errorf("shouldWatch(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestParseHint(t *testing.T) {
	tests := []struct {
		spec    string
		name    string
		value   any
		wantErr bool
	}{
		{"string:custom-sound:message", "custom-sound", "message", false},
		{"string:custom-sound:/path/to:file.ogg", "custom-sound", "/path/to:file.ogg", false},
		{"boolean:resident:true", "resident", true, false},
		{"int:urgency:5", "urgency", int32(5), false},
		{"double:volume:0.5", "volume", 0.5, false},
		{"byte:level:200", "level", byte(200), false},
		{"variant:wrapped:value", "wrapped", dbus.MakeVariant("value"), false},
		{"string:custom-sound", "", nil, true},
		{"bool:resident:true", "", nil, true},
		{"boolean:resident:maybe", "", nil, true},
		{"int:urgency:high", "", nil, true},
		{"byte:level:300", "", nil, true},
	}

	for _, tt := range tests {
		name, variant, err := parseHint(tt.spec)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseHint(%q): expected error, got none", tt.spec)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseHint(%q): unexpected error: %v", tt.spec, err)
			continue
		}
		if name != tt.name {
			t.Errorf("parseHint(%q) name = %q, want %q", tt.spec, name, tt.name)
		}
		if variant.Value() != tt.value {
			t.Errorf("parseHint(%q) value = %v, want %v", tt.spec, variant.Value(), tt.value)
		}
	}
}

func TestParseUrgency(t *testing.T) {
	tests := []struct {
		level   string
		want    byte
		wantErr bool
	}{
		{"low", 0, false},
		{"normal", 1, false},
		{"critical", 2, false},
		{"", 0, true},
		{"high", 0, true},
	}

	for _, tt := range tests {
		got, err := parseUrgency(tt.level)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseUrgency(%q): expected error, got none", tt.level)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseUrgency(%q): unexpected error: %v", tt.level, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseUrgency(%q) = %d, want %d", tt.level, got, tt.want)
		}
	}
}
