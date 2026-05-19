//go:build windows

package nodetools

func shellCommandArgs(command string) []string {
	return []string{
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		command,
	}
}
