package main

func main() {
	cp := Copy{
		TargetDevice: "/dev/f1",
		TargetPath:   "/f",
	}

	cp.Mode = "copy"

	InstallUpdate(cp)
}
