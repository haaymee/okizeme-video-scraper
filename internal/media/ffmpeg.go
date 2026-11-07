package media

import "os/exec"

func BuildConvertCmd(inputPath, outputPath string) *exec.Cmd {
	return exec.Command("ffmpeg",
		"-i", inputPath,
		"-c:v", "libvpx-vp9",
		"-crf", "32",
		"-b:v", "0",
		outputPath,
	)
}

func ConvertMp4ToWebm(inputPath, outputPath string) error {
	return BuildConvertCmd(inputPath, outputPath).Run()
}
