package platformutil

import "testing"

func TestFindPythonCommandsPrefersPython(t *testing.T) {
	lookPath := func(file string) (string, error) {
		if file == "python" {
			return "/bin/python", nil
		}
		return "", errNotFound
	}

	commands, err := findPythonCommands(lookPath, "linux")
	if err != nil {
		t.Fatalf("findPythonCommands returned error: %v", err)
	}

	if len(commands) != 1 || commands[0].Executable != "/bin/python" || len(commands[0].Args) != 0 {
		t.Fatalf("unexpected commands: %+v", commands)
	}
}

func TestFindPythonCommandsFallsBackToPyLauncher(t *testing.T) {
	lookPath := func(file string) (string, error) {
		switch file {
		case "python":
			return "", errNotFound
		case "py":
			return "C:/Windows/py.exe", nil
		default:
			return "", errNotFound
		}
	}

	commands, err := findPythonCommands(lookPath, "windows")
	if err != nil {
		t.Fatalf("findPythonCommands returned error: %v", err)
	}

	if len(commands) != 2 {
		t.Fatalf("expected two commands, got %d", len(commands))
	}

	if commands[0].Executable != "C:/Windows/py.exe" || len(commands[0].Args) != 1 || commands[0].Args[0] != "-3" {
		t.Fatalf("unexpected first command: %+v", commands[0])
	}

	if commands[1].Executable != "C:/Windows/py.exe" || len(commands[1].Args) != 0 {
		t.Fatalf("unexpected second command: %+v", commands[1])
	}
}

func TestFindPythonCommandsMissing(t *testing.T) {
	lookPath := func(string) (string, error) {
		return "", errNotFound
	}

	if _, err := findPythonCommands(lookPath, "linux"); err == nil {
		t.Fatal("expected error when python is missing")
	}
}

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (*notFoundError) Error() string        { return "not found" }
func (*notFoundError) Is(target error) bool { return target == errNotFound }
