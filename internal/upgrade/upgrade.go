// Package upgrade implements OTA (over-the-air) self-upgrade for the 2Panel
// single-binary distribution: it checks GitHub Releases for a newer version,
// downloads the asset for the current platform, verifies its SHA-256 and
// minisign signature, atomically swaps the running binary and restarts the
// service.
package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	minisign "github.com/jedisct1/go-minisign"
	"golang.org/x/mod/semver"

	"github.com/2panel-dev/2panel/internal/dto"
)

// ---------------------------------------------------------------------------
// 发布源配置：与 install.sh 顶部的 GITHUB_OWNER / GITHUB_REPO 保持一致。
// ---------------------------------------------------------------------------
const (
	GitHubOwner = "meimolihan"
	GitHubRepo  = "2Panel"

	// AssetName is the release asset name pattern, e.g. 2panel_linux_amd64.
	AssetName = "2panel_linux_%s"
)

// MinisignPublicKey 是发布资产签名所用的公钥（方案 D，防供应链篡改）。
// 将 `minisign -G` 生成的公钥文件（minisign.pub，含 untrusted comment 行的
// 完整两行内容）粘贴到此处即可启用签名校验，留空则仅做 SHA-256 完整性校验。
const MinisignPublicKey = ""

var (
	currentVersion = "v0.0.0"
	currentBuild   = "dev"
)

// SetVersion injects the ldflags-provided version/build at startup.
func SetVersion(version, build string) {
	currentVersion = version
	currentBuild = build
}

// CurrentVersion returns the running version (e.g. v1.2.3).
func CurrentVersion() string { return currentVersion }

// CurrentBuild returns the build id, or "dev" for local builds.
func CurrentBuild() string { return currentBuild }

// Enabled reports whether OTA upgrades are possible for this build. Dev builds
// and versions without a "v" release prefix must be updated manually.
func Enabled() bool {
	return currentBuild != "dev" && strings.HasPrefix(currentVersion, "v") && semver.IsValid(currentVersion)
}

// ---------------------------------------------------------------------------
// 更新检查：GitHub API releases/latest + 语义化版本比较（结果缓存 10 分钟）。
// ---------------------------------------------------------------------------

type release struct {
	Tag         string
	Body        string
	PublishedAt string
}

var (
	checkMu    sync.Mutex
	checkCache *dto.UpdateInfo
	checkTime  time.Time
)

const checkCacheTTL = 10 * time.Minute

// Check returns whether a newer release exists. It is safe for concurrent use.
func Check() (*dto.UpdateInfo, error) {
	checkMu.Lock()
	defer checkMu.Unlock()

	if checkCache != nil && time.Since(checkTime) < checkCacheTTL {
		return checkCache, nil
	}

	info := &dto.UpdateInfo{Current: currentVersion, Updatable: Enabled()}
	// 始终查询最新发布：开发构建（build=dev）也能看到更新提示，
	// 只是不能在线升级（Updatable=false 时前端会提示手动重新部署）。
	latest, err := fetchLatestRelease()
	if err != nil {
		return nil, err
	}
	info.Latest = latest.Tag
	info.PublishedAt = latest.PublishedAt
	info.Changelog = latest.Body
	if semver.IsValid(latest.Tag) && semver.Compare(latest.Tag, currentVersion) > 0 {
		info.HasUpdate = true
	}

	checkCache, checkTime = info, time.Now()
	return info, nil
}

func fetchLatestRelease() (*release, error) {
	// 优先 GitHub API（可识别草稿与预发布）；被限流（403/429）或网络失败时
	// 自动降级到 releases.atom 订阅源（不受 API 限流影响）。
	r, err := fetchLatestReleaseAPI()
	if err == nil {
		return r, nil
	}
	ra, aerr := fetchLatestReleaseAtom()
	if aerr != nil {
		return nil, fmt.Errorf("%v（Atom 降级读取也失败：%v）", err, aerr)
	}
	return ra, nil
}

func fetchLatestReleaseAPI() (*release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", GitHubOwner, GitHubRepo)
	resp, err := get(url, 15*time.Second)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// 尚未发布任何 release
	if resp.StatusCode == http.StatusNotFound {
		return &release{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 返回 %d", resp.StatusCode)
	}
	var raw struct {
		TagName     string `json:"tag_name"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		Draft       bool   `json:"draft"`
		Prerelease  bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	// 跳过草稿与预发布
	if raw.Draft || raw.Prerelease {
		return &release{}, nil
	}
	return &release{Tag: raw.TagName, Body: raw.Body, PublishedAt: raw.PublishedAt}, nil
}

func fetchLatestReleaseAtom() (*release, error) {
	url := fmt.Sprintf("https://github.com/%s/%s/releases.atom", GitHubOwner, GitHubRepo)
	resp, err := get(url, 15*time.Second)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub Atom 返回 %d", resp.StatusCode)
	}
	var feed struct {
		Entries []struct {
			Title     string `xml:"title"`
			Published string `xml:"published"`
			Content   string `xml:"content"`
			Link      struct {
				Href string `xml:"href,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, err
	}
	if len(feed.Entries) == 0 {
		return &release{}, nil
	}
	e := feed.Entries[0]
	tag := strings.TrimSpace(e.Title)
	if href := e.Link.Href; href != "" {
		// https://github.com/owner/repo/releases/tag/v1.2.3
		if i := strings.Index(href, "/releases/tag/"); i >= 0 {
			tag = strings.TrimSpace(href[i+len("/releases/tag/"):])
		}
	}
	return &release{Tag: tag, Body: stripHTML(e.Content), PublishedAt: e.Published}, nil
}

func stripHTML(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// 升级执行：下载 → SHA-256 校验 → minisign 验签 → 备份 → 原子替换 → 重启。
// ---------------------------------------------------------------------------

type upgradeState struct {
	mu       sync.Mutex
	running  bool
	status   string // idle|downloading|verifying|swapping|restarting|done|error
	newVer   string
	logLines []string
}

var st upgradeState

// Status returns a snapshot for the frontend progress panel.
func Status() dto.UpgradeStatus {
	st.mu.Lock()
	defer st.mu.Unlock()
	return dto.UpgradeStatus{
		Running:    st.running,
		State:      st.status,
		NewVersion: st.newVer,
		Log:        append([]string(nil), st.logLines...),
	}
}

func setStatus(s string) {
	st.mu.Lock()
	st.status = s
	st.mu.Unlock()
}

func logf(format string, args ...interface{}) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.logLines = append(st.logLines, time.Now().Format("15:04:05")+" "+fmt.Sprintf(format, args...))
	if len(st.logLines) > 200 {
		st.logLines = st.logLines[len(st.logLines)-200:]
	}
}

// Perform starts the upgrade in a background goroutine and returns immediately
// so the HTTP request does not block for the whole download. Progress is
// reported through Status(). An error is returned only when an upgrade is
// already running or the build does not support OTA.
func Perform() error {
	st.mu.Lock()
	if st.running {
		st.mu.Unlock()
		return fmt.Errorf("升级已在执行中，请稍候")
	}
	if !Enabled() {
		st.mu.Unlock()
		return fmt.Errorf("当前为开发构建（build=dev），不支持在线升级，请手动重新部署")
	}
	st.running = true
	st.status = "downloading"
	st.newVer = ""
	st.logLines = nil
	st.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				st.mu.Lock()
				st.running = false
				st.status = "error"
				st.mu.Unlock()
				logf("升级异常终止：%v", r)
			}
		}()
		if err := doUpgrade(); err != nil {
			st.mu.Lock()
			st.running = false
			st.status = "error"
			st.mu.Unlock()
			logf("升级失败：%v", err)
			return
		}
		st.mu.Lock()
		st.status = "restarting"
		st.mu.Unlock()
		logf("升级成功，正在重启服务…")
		restartService()
	}()
	return nil
}

// Restart 手动重启服务（systemd 或 nohup 后台模式）。
// 与升级互斥：升级进行中拒绝重启，手动重启挂起时也拒绝升级。
func Restart() error {
	st.mu.Lock()
	if st.running {
		st.mu.Unlock()
		return fmt.Errorf("升级正在进行中，无法重启，请稍候")
	}
	if st.status == "restarting" {
		st.mu.Unlock()
		return fmt.Errorf("服务正在重启中，请稍候")
	}
	st.status = "restarting"
	st.mu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logf("重启异常终止：%v", r)
			}
		}()
		logf("收到手动重启请求")
		restartService()
	}()
	return nil
}

func doUpgrade() error {
	info, err := Check()
	if err != nil {
		return err
	}
	if !info.HasUpdate {
		return fmt.Errorf("当前已是最新版本 %s", currentVersion)
	}
	st.mu.Lock()
	st.newVer = info.Latest
	st.mu.Unlock()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	base := fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/%s",
		GitHubOwner, GitHubRepo, fmt.Sprintf(AssetName, runtime.GOARCH))

	logf("开始升级到 %s", info.Latest)
	logf("下载 %s", base)
	tmp, err := os.CreateTemp(filepath.Dir(cfg.BinPath), ".2panel-update-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := downloadToFile(base, tmp); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// 1. SHA-256 完整性校验
	setStatus("verifying")
	logf("校验 SHA-256…")
	sumText, err := fetchText(base+".sha256", 30*time.Second)
	if err != nil {
		return fmt.Errorf("下载校验文件失败：%v", err)
	}
	fields := strings.Fields(sumText)
	if len(fields) == 0 {
		return fmt.Errorf("校验文件格式无效")
	}
	if sum, err := fileSHA256(tmp.Name()); err != nil {
		return err
	} else if !strings.EqualFold(sum, fields[0]) {
		return fmt.Errorf("SHA-256 校验失败（预期 %s，实际 %s），已中止", fields[0], sum)
	}
	logf("SHA-256 校验通过")

	// 2. minisign 签名校验（方案 D）
	if MinisignPublicKey != "" {
		if err := verifyMinisign(tmp.Name(), base+".minisig"); err != nil {
			return err
		}
		logf("minisign 签名校验通过")
	} else {
		logf("未配置 minisign 公钥，跳过签名校验")
	}

	// 3. 备份当前二进制，然后原子替换
	setStatus("swapping")
	logf("备份当前二进制到 %s.bak", cfg.BinPath)
	if err := copyFile(cfg.BinPath, cfg.BinPath+".bak"); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		return err
	}
	logf("替换二进制 %s", cfg.BinPath)
	if err := os.Rename(tmp.Name(), cfg.BinPath); err != nil {
		return fmt.Errorf("替换二进制失败：%v", err)
	}
	logf("二进制替换完成")
	return nil
}

func verifyMinisign(binPath, sigURL string) error {
	pub, err := minisign.DecodePublicKey(MinisignPublicKey)
	if err != nil {
		return fmt.Errorf("解析 minisign 公钥失败：%v", err)
	}
	sigText, err := fetchText(sigURL, 30*time.Second)
	if err != nil {
		return fmt.Errorf("下载签名文件失败：%v", err)
	}
	sig, err := minisign.DecodeSignature(sigText)
	if err != nil {
		return fmt.Errorf("解析签名失败：%v", err)
	}
	message, err := os.ReadFile(binPath)
	if err != nil {
		return err
	}
	ok, err := pub.Verify(message, sig)
	if err != nil || !ok {
		return fmt.Errorf("签名校验失败，发布资产可能已被篡改，已中止（%v）", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 服务重启：优先 systemd，否则 nohup 后台模式（延迟自杀 + 拉起新进程）。
// ---------------------------------------------------------------------------

func restartService() {
	cfg, err := loadConfig()
	if err != nil {
		logf("重启失败：无法确定服务配置：%v", err)
		return
	}
	if isSystemd() {
		time.Sleep(500 * time.Millisecond)
		go func() { _ = exec.Command("systemctl", "restart", "2panel").Run() }()
		logf("已触发 systemctl restart 2panel")
		return
	}
	// 后台运行模式：等待响应返回后杀掉自身，再由 helper 拉起新二进制。
	logf("以 nohup 模式重启（port=%s data=%s）", cfg.Port, cfg.DataDir)
	script := fmt.Sprintf(
		"sleep 1; kill -TERM %d 2>/dev/null; sleep 1; nohup %q -port %q -data %q >> %q/2panel.log 2>&1 &",
		os.Getpid(), cfg.BinPath, cfg.Port, cfg.DataDir, cfg.DataDir)
	cmd := exec.Command("sh", "-c", script)
	// 尽量脱离当前进程组，避免旧进程退出时信号级联到 helper。
	if setsid, err := exec.LookPath("setsid"); err == nil {
		cmd = exec.Command(setsid, "sh", "-c", script)
	}
	_ = cmd.Start()
}

func isSystemd() bool {
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return false
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	if _, err := os.Stat("/etc/systemd/system/2panel.service"); err != nil {
		return false
	}
	return true
}

// installConfig mirrors the installation record written by install.sh.
type installConfig struct {
	BinPath string
	Port    string
	DataDir string
}

// loadConfig resolves the running installation. Priority: /etc/2panel/config,
// then the current process command line, then sane defaults.
func loadConfig() (installConfig, error) {
	exe, err := os.Executable()
	if err != nil {
		return installConfig{}, err
	}
	cfg := installConfig{BinPath: exe, Port: flagFromArgs("-port", "8080"), DataDir: filepath.Join(filepath.Dir(exe), "data")}

	if b, err := os.ReadFile("/etc/2panel/config"); err == nil {
		vals := make(map[string]string)
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if i := strings.Index(line, "="); i > 0 {
				vals[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
			}
		}
		if v := vals["BIN_PATH"]; v != "" {
			cfg.BinPath = v
		}
		if v := vals["PORT"]; v != "" {
			cfg.Port = v
		}
		if v := vals["DATA_DIR"]; v != "" {
			cfg.DataDir = v
		}
	}
	return cfg, nil
}

func flagFromArgs(name, def string) string {
	args := os.Args[1:]
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			return args[i+1]
		}
		if strings.HasPrefix(args[i], name+"=") {
			return strings.TrimPrefix(args[i], name+"=")
		}
	}
	return def
}

// ---------------------------------------------------------------------------
// 下载与文件工具。
// ---------------------------------------------------------------------------

func get(url string, timeout time.Duration) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "2Panel-upgrade")
	client := &http.Client{Timeout: timeout}
	return client.Do(req)
}

func downloadToFile(url string, f *os.File) error {
	resp, err := get(url, 20*time.Minute)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败：HTTP %d", resp.StatusCode)
	}
	// 防御：拒绝超大文件
	if resp.ContentLength > 300<<20 {
		return fmt.Errorf("下载文件过大（%d 字节），已中止", resp.ContentLength)
	}
	_, err = io.Copy(f, resp.Body)
	return err
}

func fetchText(url string, timeout time.Duration) (string, error) {
	resp, err := get(url, timeout)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return string(b), err
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
