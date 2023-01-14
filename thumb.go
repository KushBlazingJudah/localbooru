package localbooru

// TODO: Do this inside of Go instead of calling upon ImageMagick?

import (
	"fmt"
	"os/exec"
)

const (
	thumbWidth  = 195
	thumbHeight = 185
)

func thumbify(src, dst string) error {
	cmd := exec.Command("convert", fmt.Sprintf("%s[0]", src), "-thumbnail", fmt.Sprintf("%dx%d", thumbWidth, thumbHeight), dst)
	return cmd.Run()
}
