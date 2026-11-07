package media

import "testing"

func argIndex(args []string, s string) int {
	for i, a := range args {
		if a == s {
			return i
		}
	}
	return -1
}

// Guards against the ffmpeg argument-order bug: output options (-c:v, -crf,
// -b:v) must come AFTER -i, otherwise ffmpeg treats them as input options and
// fails with EINVAL ("Move this option before the file it belongs to").
func TestBuildConvertCmdArgOrder(t *testing.T) {
	cmd := BuildConvertCmd("in.mp4", "out.webm")
	args := cmd.Args

	iInput := argIndex(args, "-i")
	iCodec := argIndex(args, "-c:v")
	if iInput < 0 || iCodec < 0 {
		t.Fatalf("missing -i or -c:v in %v", args)
	}
	if iCodec < iInput {
		t.Fatalf("-c:v must come after -i; got %v", args)
	}
	if iInput+1 >= len(args) || args[iInput+1] != "in.mp4" {
		t.Fatalf("expected input path right after -i; got %v", args)
	}
	if args[len(args)-1] != "out.webm" {
		t.Fatalf("expected output path last; got %v", args)
	}
}
