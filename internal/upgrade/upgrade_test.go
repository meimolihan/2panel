package upgrade

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCandidateURLs(t *testing.T) {
	raw := "https://github.com/owner/repo/releases/latest/download/2panel_linux_amd64"
	got := candidateURLs(raw)

	if len(got) != len(githubMirrors)+1 {
		t.Fatalf("候选数 = %d，期望 %d", len(got), len(githubMirrors)+1)
	}
	for i, prefix := range githubMirrors {
		if !strings.HasPrefix(got[i], prefix) || !strings.HasSuffix(got[i], raw) {
			t.Errorf("候选[%d] = %q，应为镜像 %q + 原 URL", i, got[i], prefix)
		}
	}
	if got[len(got)-1] != raw {
		t.Errorf("最后一项应回退原地址，实际 %q", got[len(got)-1])
	}
}

func TestCheckMagic(t *testing.T) {
	dir := t.TempDir()

	elf := filepath.Join(dir, "elf.bin")
	if err := os.WriteFile(elf, []byte{0x7f, 0x45, 0x4c, 0x46, 0x02, 0x01}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := checkMagic(elf, 0x7f, 0x45, 0x4c, 0x46); err != nil {
		t.Errorf("合法 ELF 应通过魔数校验：%v", err)
	}

	html := filepath.Join(dir, "error.html")
	if err := os.WriteFile(html, []byte("<html>404 Not Found</html>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := checkMagic(html, 0x7f, 0x45, 0x4c, 0x46); err == nil {
		t.Error("HTML 错误页应被魔数校验拒绝")
	}

	short := filepath.Join(dir, "short.bin")
	if err := os.WriteFile(short, []byte{0x7f, 0x45}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := checkMagic(short, 0x7f, 0x45, 0x4c, 0x46); err == nil {
		t.Error("截断文件应被魔数校验拒绝")
	}
}

func TestShortHash(t *testing.T) {
	sum := hex.EncodeToString(make([]byte, 32))
	if got := shortHash(sum); len(got) != 12 {
		t.Errorf("shortHash 应截为 12 位，实际 %d 位：%q", len(got), got)
	}
	if got := shortHash("abc"); got != "abc" {
		t.Errorf("短字符串应原样返回，实际 %q", got)
	}
}
