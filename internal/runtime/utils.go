package runtime

import (
	"fmt"
	"math/rand"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"nedgentic/internal/output"
)

type StepTimer struct {
	start time.Time
	step  int
}

func NewStepTimer() *StepTimer {
	return &StepTimer{start: time.Now()}
}

func (timer *StepTimer) Elapsed() time.Duration {
	if timer == nil {
		return 0
	}
	return time.Since(timer.start)
}

func (timer *StepTimer) Run(label string, fn func() error) error {
	if timer == nil {
		return fn()
	}
	timer.step++
	stepStart := time.Now()
	err := fn()
	elapsed := time.Since(stepStart)
	if err != nil {
		fmt.Println(output.Format(fmt.Sprintf("%s: Failed [%.3fs]", label, elapsed.Seconds())))
		return err
	}
	fmt.Println(output.Format(fmt.Sprintf("%s: [%.3fs]", label, elapsed.Seconds())))
	return nil
}

func ResolveHostUser() (*HostUser, error) {
	if sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER")); sudoUser != "" {
		usr, err := user.Lookup(sudoUser)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve SUDO_USER %q: %v", sudoUser, err)
		}
		return parseHostUser(usr)
	}
	usr, err := user.Current()
	if err != nil {
		return nil, err
	}
	return parseHostUser(usr)
}

func parseHostUser(usr *user.User) (*HostUser, error) {
	if usr == nil {
		return nil, fmt.Errorf("user is nil")
	}
	if usr.Username == "" {
		return nil, fmt.Errorf("username is empty")
	}
	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		return nil, fmt.Errorf("failed to parse uid: %v", err)
	}
	gid, err := strconv.Atoi(usr.Gid)
	if err != nil {
		return nil, fmt.Errorf("failed to parse gid: %v", err)
	}
	home := usr.HomeDir
	if home == "" {
		return nil, fmt.Errorf("user home directory is empty")
	}
	return &HostUser{Name: usr.Username, UID: uid, GID: gid, Home: home}, nil
}

var (
	nameRandMu = &sync.Mutex{}
	nameRand   = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func GenerateHostName() string {
	adjectives := []string{
		"brave", "calm", "clear", "fast", "gentle", "keen", "lucky", "mighty",
		"rapid", "silent", "solid", "steady", "swift", "bright", "bold", "sharp",
	}
	nouns := []string{
		"ant", "bear", "cat", "eagle", "fox", "hawk", "lion", "owl",
		"puma", "ray", "seal", "wolf", "yak", "orca", "lynx", "tiger",
	}
	nameRandMu.Lock()
	adj := adjectives[nameRand.Intn(len(adjectives))]
	noun := nouns[nameRand.Intn(len(nouns))]
	suffix := nameRand.Intn(0x10000)
	nameRandMu.Unlock()
	return fmt.Sprintf("%s-%s-%04x", adj, noun, suffix)
}

// FormatBytes renders a byte count with binary units (KiB, MiB, ...).
func FormatBytes(value int64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%dB", value)
	}
	div, exp := int64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(value)/float64(div), "KMGTPE"[exp])
}

func ParseSize(size string) (int64, error) {
	s := strings.TrimSpace(strings.ToUpper(size))
	if s == "" {
		return 0, nil
	}
	var multiplier int64 = 1
	switch {
	case strings.HasSuffix(s, "KB") || strings.HasSuffix(s, "K"):
		multiplier = 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "KB"), "K")
	case strings.HasSuffix(s, "MB") || strings.HasSuffix(s, "M"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "MB"), "M")
	case strings.HasSuffix(s, "GB") || strings.HasSuffix(s, "G"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "GB"), "G")
	case strings.HasSuffix(s, "TB") || strings.HasSuffix(s, "T"):
		multiplier = 1024 * 1024 * 1024 * 1024
		s = strings.TrimSuffix(strings.TrimSuffix(s, "TB"), "T")
	case strings.HasSuffix(s, "B"):
		s = strings.TrimSuffix(s, "B")
	}
	val, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %v", err)
	}
	return val * multiplier, nil
}

func CheckDiskSpace(path string, requiredBytes int64) error {
	if requiredBytes <= 0 {
		return nil
	}

	// Ensure path exists or use parent
	checkPath := path
	if _, err := os.Stat(checkPath); os.IsNotExist(err) {
		checkPath = os.TempDir() // Fallback to temp dir if path doesn't exist yet
	}

	return checkDiskSpace(checkPath, requiredBytes)
}

func checkDiskSpace(path string, requiredBytes int64) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return fmt.Errorf("failed to check disk space: %v", err)
	}

	// Available blocks * block size
	available := int64(stat.Bavail) * int64(stat.Bsize)
	if available < requiredBytes {
		return fmt.Errorf("insufficient disk space on %s: available %s, required %s",
			path, formatSize(available), formatSize(requiredBytes))
	}
	return nil
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
