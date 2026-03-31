package shell

import (
	"strings"
	"testing"
)

func TestInitScript_Zsh(t *testing.T) {
	script, err := InitScript("zsh")
	if err != nil {
		t.Fatalf("InitScript zsh: %v", err)
	}
	if !strings.Contains(script, "gtf()") {
		t.Error("expected shell function definition")
	}
	if !strings.Contains(script, "cd") {
		t.Error("expected cd command")
	}
}

func TestInitScript_Bash(t *testing.T) {
	script, err := InitScript("bash")
	if err != nil {
		t.Fatalf("InitScript bash: %v", err)
	}
	if !strings.Contains(script, "gtf()") {
		t.Error("expected shell function definition")
	}
}

func TestInitScript_Fish(t *testing.T) {
	script, err := InitScript("fish")
	if err != nil {
		t.Fatalf("InitScript fish: %v", err)
	}
	if !strings.Contains(script, "function gtf") {
		t.Error("expected fish function definition")
	}
}

func TestInitScript_Unknown(t *testing.T) {
	_, err := InitScript("powershell")
	if err == nil {
		t.Error("expected error for unknown shell")
	}
}
