package localbooru

import (
	"fmt"
	"os/exec"
	"path"
	"os"
)

const (
	thumbWidth  = 195
	thumbHeight = 185
)

func thumbify(src, dst string) error {
	// TODO: Do this inside of Go instead of calling upon ImageMagick?
	cmd := exec.Command("convert", fmt.Sprintf("%s[0]", src), "-thumbnail", fmt.Sprintf("%dx%d", thumbWidth, thumbHeight), dst)
	return cmd.Run()
}

func binHash(h string) string {
	return h[:2] + "/" + h[2:4]
}

func deleteFile(base, hash, ext string) error {
	pth := path.Join(base, binHash(hash), hash+"."+ext)
	tpth := path.Join(base, binHash(hash), hash+".thumb.jpg")

	fmt.Println(pth, tpth)

	if err := os.Remove(tpth); err != nil {
		return err
	}
	return os.Remove(pth)
}
