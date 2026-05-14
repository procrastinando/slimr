package converter

import (
	"bytes"
	"fmt"
	"os/exec"
)

func TransferMetadata(original, compressed string) error {
	cmd := exec.Command("exiftool",
		"-tagsfromfile", original,
		"-all:all",
		"-overwrite_original",
		compressed,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exiftool: %w\n%s", err, stderr.String())
	}
	return nil
}
