package ping

import "testing"

func TestCommands(t *testing.T) {
	commands := New().Commands()
	if len(commands) != 1 {
		t.Fatalf("commands len = %d", len(commands))
	}
	if commands[0].CommandName() != CommandName {
		t.Fatalf("command name = %q", commands[0].CommandName())
	}
}

func TestResponse(t *testing.T) {
	response := Response()
	if response.Content != "jihyooo ❤️" {
		t.Fatalf("response content = %q", response.Content)
	}
}
