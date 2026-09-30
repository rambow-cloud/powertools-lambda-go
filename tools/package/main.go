// Package main validates cross-compiled Lambda executables and creates ZIP archives.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/elf"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	for _, arch := range []string{"amd64", "arm64"} {
		if err := pack(arch); err != nil {
			log.Fatal(err)
		}
	}
}
func pack(arch string) error {
	path := filepath.Join("dist", arch, "bootstrap")
	binary, err := elf.Open(path)
	if err != nil {
		return err
	}
	expected := elf.EM_X86_64
	if arch == "arm64" {
		expected = elf.EM_AARCH64
	}
	if binary.Machine != expected {
		binary.Close()
		return fmt.Errorf("wrong ELF machine for %s", arch)
	}
	for _, p := range binary.Progs {
		if p.Type == elf.PT_INTERP {
			binary.Close()
			return fmt.Errorf("%s requires a dynamic loader", path)
		}
	}
	binary.Close()
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return err
	}
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	if settings["CGO_ENABLED"] != "0" || settings["GOOS"] != "linux" || settings["GOARCH"] != arch {
		return fmt.Errorf("unexpected build settings for %s: %v", path, settings)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	zipPath := filepath.Join("dist", "lambda-"+arch+".zip")
	out, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(out)
	header := &zip.FileHeader{Name: "bootstrap", Method: zip.Deflate}
	header.SetMode(0755)
	header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
	entry, err := writer.CreateHeader(header)
	if err == nil {
		_, err = entry.Write(data)
	}
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	check, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer check.Close()
	if len(check.File) != 1 || check.File[0].Name != "bootstrap" || check.File[0].Mode().Perm() != 0755 {
		return fmt.Errorf("invalid Lambda archive: %s", zipPath)
	}
	fmt.Printf("%s: Linux %s, CGO_ENABLED=0, static ELF, bootstrap mode 0755, binary SHA256 %x\n", zipPath, arch, sha256.Sum256(data))
	return nil
}
