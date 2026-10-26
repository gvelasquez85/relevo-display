package desktop

import "strconv"

// Keeping the command strategy portable lets tests catch Linux invocation
// regressions even when development happens on a different operating system.
func hyprRecorderArgs(name string, fps int) []string {
	if fps < 1 || fps > 30 {
		fps = 5
	}
	return []string{"--no-damage", "-r", strconv.Itoa(fps), "-o", name, "-c", "mjpeg", "--muxer=mjpeg", "-x", "yuvj420p", "-f", "/proc/self/fd/3"}
}
