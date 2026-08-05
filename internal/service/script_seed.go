package service

import (
	"log"

	"github.com/2panel-dev/2panel/internal/model"
	"github.com/2panel-dev/2panel/internal/repo"
)

// SettingScriptSeedVersion tracks whether the built-in scripts have been
// inserted. Seeding runs only once so user deletions are respected.
const SettingScriptSeedVersion = "script_seed_version"

type preseedScript struct {
	Name        string
	Description string
	Script      string
}

var preseedScripts = []preseedScript{
	{
		Name:        "安装 Docker",
		Description: "安装 Docker 容器引擎（自动识别系统，国内环境自动切换镜像源）",
		Script: `
#!/bin/bash

country=$(curl -s ipinfo.io/country)
echo "Current server location: $country"

if [[ "$country" == "CN" ]]; then
  bash <(curl -sSL https://linuxmirrors.cn/docker.sh)
else
  curl -fsSL https://get.docker.com -o get-docker.sh
  sudo sh get-docker.sh
fi

`},
	{
		Name:        "安装 ClamAV",
		Description: "安装 ClamAV 杀毒软件（支持 Ubuntu/Debian/CentOS/Alpine/Arch）",
		Script: `
#!/bin/bash

# Install ClamAV
# Support Ubuntu/Debian/CentOS/RHEL/Alpine/Arch Linux

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
    elif type lsb_release >/dev/null 2>&1; then
        OS=$(lsb_release -si | tr '[:upper:]' '[:lower:]')
        VERSION=$(lsb_release -sr)
    elif [ -f /etc/redhat-release ]; then
        OS="rhel"
        VERSION=$(grep -oE '[0-9]+\.[0-9]+' /etc/redhat-release)
    elif [ -f /etc/alpine-release ]; then
        OS="alpine"
        VERSION=$(cat /etc/alpine-release)
    else
        OS=$(uname -s | tr '[:upper:]' '[:lower:]')
        VERSION=$(uname -r)
    fi
}

install_clamav() {
    echo -e "${GREEN}Detected system: $OS $VERSION${NC}"

    case "$OS" in
        ubuntu|debian)
            apt-get update
            apt-get install -y clamav clamav-daemon clamav-freshclam
            ;;
        centos|rhel|fedora)
            if [ "$OS" = "rhel" ] && [ "${VERSION%%.*}" -ge 8 ]; then
                dnf install -y epel-release
                dnf install -y clamav clamd clamav-update
            else
                yum install -y epel-release
                yum install -y clamav clamd clamav-update
            fi
            ;;
        alpine)
            apk add --no-cache clamav clamav-libunrar clamav-daemon clamav-freshclam
            ;;
        arch)
            pacman -Sy --noconfirm clamav
            ;;
        *)
            echo -e "${RED}Unsupported system${NC}"
            exit 1
            ;;
    esac
}

configure_clamd() {
    echo -e "${GREEN}Configure clamd...${NC}"
    
    CLAMD_CONF=""
    if [ -f "/etc/clamd.d/scan.conf" ]; then
        CLAMD_CONF="/etc/clamd.d/scan.conf"
    elif [ -f "/etc/clamav/clamd.conf" ]; then
        CLAMD_CONF="/etc/clamav/clamd.conf"
    else
        echo "clamd configuration file not found, please manually configure"
        exit 1
    fi
    cp "$CLAMD_CONF" "$CLAMD_CONF.bak"

    sed -i -E 's|^#\s?LogFileMaxSize\s+.*|LogFileMaxSize 2M|' "$CLAMD_CONF"
    sed -i -E 's|^#\s?PidFile\s+.*|PidFile /run/clamd.scan/clamd.pid|' "$CLAMD_CONF"
    sed -i -E 's|^#\s?DatabaseDirectory\s+.*|DatabaseDirectory /var/lib/clamav|' "$CLAMD_CONF"
    sed -i -E 's|^#\s?LocalSocket\s+.*|LocalSocket /run/clamd.scan/clamd.sock|' "$CLAMD_CONF"
}

configure_freshclam() {
    echo -e "${GREEN}Configure freshclam...${NC}"
    
    FRESHCLAM_CONF=""
    if [ -f "/etc/freshclam.conf" ]; then
        FRESHCLAM_CONF="/etc/freshclam.conf"
    elif [ -f "/etc/clamav/freshclam.conf" ]; then
        FRESHCLAM_CONF="/etc/clamav/freshclam.conf"
    else
        echo "freshclam configuration file not found, please manually configure"
        exit 1
    fi
    cp "$FRESHCLAM_CONF" "$FRESHCLAM_CONF.bak"

    sed -i -E 's|^#\s?DatabaseDirectory\s+.*|DatabaseDirectory /var/lib/clamav|' "$FRESHCLAM_CONF"
    sed -i -E 's|^#\s?PidFile\s+.*|PidFile /var/run/freshclam.pid|' "$FRESHCLAM_CONF"
    sed -i '/^DatabaseMirror/d' "$FRESHCLAM_CONF"
    echo "DatabaseMirror database.clamav.net" | sudo tee -a "$FRESHCLAM_CONF"
    sed -i -E 's|^#\s?Checks\s+.*|Checks 12|' "$FRESHCLAM_CONF"
}

download_database() {
    systemctl stop clamav-freshclam
    echo -e "${GREEN}The virus database starts to download...${NC}"
    
    MAX_RETRIES=5
    RETRY_DELAY=60
    ATTEMPT=1
    
    while [ $ATTEMPT -le $MAX_RETRIES ]; do
        echo -e "${YELLOW}Try $ATTEMPT/$MAX_RETRIES: run freshclam...${NC}"
        
        if freshclam --verbose; then
            echo -e "${GREEN}Download successfully${NC}"
            return 0
        fi
        
        if [ $ATTEMPT -lt $MAX_RETRIES ]; then
            echo -e "${YELLOW}Download failed, wait $RETRY_DELAY seconds and try again...${NC}"
            sleep $RETRY_DELAY
        fi
        
        ATTEMPT=$((ATTEMPT+1))
    done
    
    echo -e "${RED}Error: Unable to download virus database after $MAX_RETRIES attempt${NC}" >&2
    exit 1
}

start_services() {
    echo -e "${GREEN}Start ClamAV...${NC}"
    
    case "$OS" in
        ubuntu|debian)
            systemctl enable --now clamav-daemon
            systemctl enable --now clamav-freshclam
            ;;
        centos|rhel|fedora)
            systemctl enable --now clamd@scan
            systemctl enable --now clamav-freshclam
            ;;
        alpine)
            rc-update add clamd boot
            rc-update add freshclam boot
            rc-service clamd start
            rc-service freshclam start
            ;;
        arch)
            systemctl enable --now clamav-daemon
            systemctl enable --now clamav-freshclam
            ;;
        *)
            echo -e "${YELLOW}The service cannot be started automatically. Please start manually!${NC}"
            ;;
    esac
    
   if ! command -v clamscan &> /dev/null; then
        echo -e "${RED}Install ClamAV failed${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}ClamAV is installed and started${NC}"
}

main() {
    detect_os
    install_clamav
    configure_clamd
    configure_freshclam
    download_database
    start_services
}

main "$@"
`},
	{
		Name:        "安装 Fail2ban",
		Description: "安装 Fail2ban 防暴力破解并配置 SSH 保护",
		Script: `
#!/bin/bash

# Install Fail2ban
# Support Ubuntu/Debian/CentOS/RHEL/Alpine/Arch Linux

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color


detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
    elif type lsb_release >/dev/null 2>&1; then
        OS=$(lsb_release -si | tr '[:upper:]' '[:lower:]')
        VERSION=$(lsb_release -sr)
    elif [ -f /etc/redhat-release ]; then
        OS="rhel"
        VERSION=$(grep -oE '[0-9]+\.[0-9]+' /etc/redhat-release)
    elif [ -f /etc/alpine-release ]; then
        OS="alpine"
        VERSION=$(cat /etc/alpine-release)
    else
        OS=$(uname -s | tr '[:upper:]' '[:lower:]')
        VERSION=$(uname -r)
    fi
}

install_fail2ban() {
    echo -e "${GREEN}Detected system: $OS $VERSION${NC}"

    case "$OS" in
        ubuntu|debian)
            apt-get update
            apt-get install -y fail2ban
            ;;
        centos|rhel|fedora)
            if [ "$OS" = "rhel" ] && [ "${VERSION%%.*}" -ge 8 ]; then
                dnf install -y epel-release
                dnf install -y fail2ban
            else
                yum install -y epel-release
                yum install -y fail2ban
            fi
            ;;
        alpine)
            apk add --no-cache fail2ban
            ;;
        arch)
            pacman -Sy --noconfirm fail2ban
            ;;
        *)
            echo -e "${RED}Unsupported system${NC}"
            exit 1
            ;;
    esac

    sleep 2
    if command -v systemctl &> /dev/null; then
        systemctl status fail2ban --no-pager || true
    else
        rc-service fail2ban status || true
    fi

    fail2ban-client status
}

configure_fail2ban() {
    echo -e "${GREEN}Configure Fail2ban...${NC}"
    
    FAIL2BAN_CONF="/etc/fail2ban/jail.local"
    LOG_FILE=""
    BAN_ACTION=""

    if systemctl is-active --quiet firewalld 2>/dev/null; then
        BAN_ACTION="firewallcmd-ipset"
    elif systemctl is-active --quiet ufw 2>/dev/null || service ufw status 2>/dev/null | grep -q "active"; then
        BAN_ACTION="ufw"
    else
        BAN_ACTION="iptables-allports"
    fi

    if [ -f /var/log/secure ]; then
        LOG_FILE="/var/log/secure"
    else
        LOG_FILE="/var/log/auth.log"
    fi

    cat <<EOF > "$FAIL2BAN_CONF"
#DEFAULT-START
[DEFAULT]
bantime = 600
findtime = 300
maxretry = 5
banaction = $BAN_ACTION
action = %(action_mwl)s
#DEFAULT-END

[sshd]
ignoreip = 127.0.0.1/8
enabled = true
filter = sshd
port = 22
maxretry = 5
findtime = 300
bantime = 600
banaction = $BAN_ACTION
action = %(action_mwl)s
logpath = $LOG_FILE
EOF
}

start_service() {
    echo -e "${GREEN}Start Fail2ban...${NC}"
    
    case "$OS" in
        ubuntu|debian)
            systemctl enable fail2ban
            systemctl restart fail2ban
            ;;
        centos|rhel|fedora)
            systemctl enable fail2ban
            systemctl restart fail2ban
            ;;
        alpine)
            rc-update add fail2ban
            rc-service fail2ban start
            ;;
        arch)
            systemctl enable fail2ban
            systemctl restart fail2ban
            ;;
        *)
            echo -e "${YELLOW}The service cannot be started automatically. Please start manually!${NC}"
            ;;
    esac

    if command -v systemctl &> /dev/null; then
        systemctl status fail2ban || true
    else
        rc-service fail2ban status || true
    fi

    echo -e "${GREEN}Fail2ban is installed and started${NC}"
}

main() {
    detect_os
    install_fail2ban
    configure_fail2ban
    start_service
}

main "$@"
`},
	{
		Name:        "安装 Pure-FTPd",
		Description: "安装 Pure-FTPd FTP 服务并完成基础配置",
		Script: `
#!/bin/bash

# Install Pure-FTPd
# Support Ubuntu/Debian/CentOS/RHEL/Alpine/Arch Linux

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color


detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
    elif type lsb_release >/dev/null 2>&1; then
        OS=$(lsb_release -si | tr '[:upper:]' '[:lower:]')
        VERSION=$(lsb_release -sr)
    elif [ -f /etc/redhat-release ]; then
        OS="rhel"
        VERSION=$(grep -oE '[0-9]+\.[0-9]+' /etc/redhat-release)
    elif [ -f /etc/alpine-release ]; then
        OS="alpine"
        VERSION=$(cat /etc/alpine-release)
    else
        OS=$(uname -s | tr '[:upper:]' '[:lower:]')
        VERSION=$(uname -r)
    fi
}

install_pureftpd() {
    echo -e "${GREEN}Detected system: $OS $VERSION${NC}"

    case "$OS" in
        ubuntu|debian)
            apt-get update
            apt-get install -y pure-ftpd
            ;;
        centos|rhel|fedora)
            if [ "$OS" = "rhel" ] && [ "${VERSION%%.*}" -ge 8 ]; then
                dnf install -y epel-release
                dnf install -y pure-ftpd
            else
                yum install -y epel-release
                yum install -y pure-ftpd
            fi
            ;;
        alpine)
            apk add --no-cache pure-ftpd
            ;;
        arch)
            pacman -Sy --noconfirm pure-ftpd
            ;;
        *)
            echo -e "${RED}Unsupported system${NC}"
            exit 1
            ;;
    esac

    if ! command -v pure-ftpd &> /dev/null; then
        echo -e "${RED}Install Pure-FTPd failed${NC}"
        exit 1
    fi
}

configure_pureftpd() {
    echo -e "${GREEN}Configure Pure-FTPd...${NC}"
    
    PURE_FTPD_CONF="/etc/pure-ftpd/pure-ftpd.conf"
    if [ -f "$PURE_FTPD_CONF" ]; then
        cp "$PURE_FTPD_CONF" "$PURE_FTPD_CONF.bak"
        sed -i 's/^NoAnonymous[[:space:]]\+no$/NoAnonymous yes/' "$PURE_FTPD_CONF"
        sed -i 's/^PAMAuthentication[[:space:]]\+yes$/PAMAuthentication no/' "$PURE_FTPD_CONF"
        sed -i 's/^# PassivePortRange[[:space:]]\+30000 50000$/PassivePortRange 39000 40000/' "$PURE_FTPD_CONF"
        sed -i 's/^VerboseLog[[:space:]]\+no$/VerboseLog yes/' "$PURE_FTPD_CONF"
        sed -i 's/^# PureDB[[:space:]]\+\/etc\/pure-ftpd\/pureftpd\.pdb[[:space:]]*$/PureDB \/etc\/pure-ftpd\/pureftpd.pdb/' "$PURE_FTPD_CONF"
    else
        touch /etc/pure-ftpd/pureftpd.pdb
        chmod 644 /etc/pure-ftpd/pureftpd.pdb
        echo '/etc/pure-ftpd/pureftpd.pdb' > /etc/pure-ftpd/conf/PureDB
        echo yes > /etc/pure-ftpd/conf/VerboseLog 
        echo yes > /etc/pure-ftpd/conf/NoAnonymous
        echo '39000 40000' > /etc/pure-ftpd/conf/PassivePortRange
        echo 'no' > /etc/pure-ftpd/conf/PAMAuthentication
        echo 'no' > /etc/pure-ftpd/conf/UnixAuthentication
        echo 'clf:/var/log/pure-ftpd/transfer.log' > /etc/pure-ftpd/conf/AltLog
        ln -s /etc/pure-ftpd/conf/PureDB /etc/pure-ftpd/auth/50puredb
    fi
}

start_service() {
    echo -e "${GREEN}Start Pure-FTPd...${NC}"
    
    case "$OS" in
        ubuntu|debian)
            systemctl enable pure-ftpd
            systemctl restart pure-ftpd
            ;;
        centos|rhel|fedora)
            systemctl enable pure-ftpd
            systemctl restart pure-ftpd
            ;;
        alpine)
            rc-update add pure-ftpd
            rc-service pure-ftpd start
            ;;
        arch)
            systemctl enable pure-ftpd
            systemctl restart pure-ftpd
            ;;
        *)
            echo -e "${YELLOW}The service cannot be started automatically. Please start manually!${NC}"
            ;;
    esac

    if command -v systemctl &> /dev/null; then
        systemctl status pure-ftpd || true
    else
        rc-service pure-ftpd status || true
    fi

    echo -e "${GREEN}Pure-FTPd is installed and started${NC}"
}

main() {
    detect_os
    install_pureftpd
    configure_pureftpd
    start_service
}

main "$@"
`},
	{
		Name:        "安装 Supervisor",
		Description: "安装 Supervisor 进程守护工具",
		Script: `
#!/bin/bash

# Install Supervisor
# Support Ubuntu/Debian/CentOS/RHEL/Alpine/Arch Linux

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
    elif type lsb_release >/dev/null 2>&1; then
        OS=$(lsb_release -si | tr '[:upper:]' '[:lower:]')
        VERSION=$(lsb_release -sr)
    elif [ -f /etc/redhat-release ]; then
        OS="rhel"
        VERSION=$(grep -oE '[0-9]+\.[0-9]+' /etc/redhat-release)
    elif [ -f /etc/alpine-release ]; then
        OS="alpine"
        VERSION=$(cat /etc/alpine-release)
    else
        OS=$(uname -s | tr '[:upper:]' '[:lower:]')
        VERSION=$(uname -r)
    fi
}

install_supervisor() {
    echo -e "${GREEN}Detected system: $OS $VERSION${NC}"

    case "$OS" in
        ubuntu|debian)
            apt-get update
            apt-get install -y supervisor
            ;;
        centos|rhel|fedora)
            if [ "$OS" = "rhel" ] && [ "${VERSION%%.*}" -ge 8 ]; then
                dnf install -y supervisor
            else
                yum install -y supervisor
            fi
            ;;
        alpine)
            apk add --no-cache supervisor
            mkdir -p /etc/supervisor.d
            ;;
        arch)
            pacman -Sy --noconfirm supervisor
            ;;
        *)
            echo -e "${RED}Unsupported system${NC}"
            exit 1
            ;;
    esac
}

start_service() {
    echo -e "${GREEN}Start Supervisor...${NC}"
    
    case "$OS" in
        ubuntu|debian)
            systemctl enable supervisor
            systemctl restart supervisor
            ;;
        centos|rhel|fedora)
            systemctl enable supervisor
            systemctl restart supervisor
            ;;
        alpine)
            rc-update add supervisor
            rc-service supervisor start
            ;;
        arch)
            systemctl enable supervisor
            systemctl restart supervisor
            ;;
        *)
            echo -e "${YELLOW}The service cannot be started automatically. Please start manually!${NC}"
            ;;
    esac

    if command -v systemctl &> /dev/null; then
        systemctl status supervisor || true
    else
        rc-service supervisor status || true
    fi

     echo -e "${GREEN}Supervisor is installed and started${NC}"
}

main() {
    detect_os
    install_supervisor
    start_service
}

main "$@"
`},
}

// SeedScripts populates the script library with built-in install scripts on
// the first run. The seed is tracked by a setting key so scripts deleted by
// the user are never resurrected by later restarts.
func SeedScripts() {
	if _, err := settingRepo.Get(SettingScriptSeedVersion); err == nil {
		return
	}
	for _, s := range preseedScripts {
		if _, err := scriptLibraryRepo.Get(repo.WithByName(s.Name)); err == nil {
			continue
		}
		if err := scriptLibraryRepo.Create(&model.ScriptLibrary{
			Name:        s.Name,
			Description: s.Description,
			Script:      s.Script,
		}); err != nil {
			log.Printf("seed script [%s] failed: %v", s.Name, err)
		}
	}
	_ = settingRepo.Set(SettingScriptSeedVersion, "1")
}
