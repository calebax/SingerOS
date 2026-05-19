//go:build !windows

package nodetools

func shellCommandArgs(command string) []string {
	return []string{"sh", "-lc", command}
}
