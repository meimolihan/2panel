优化 安装卸载脚本

docker pull mobufan/2panel:latest

```bash
docker pull mobufan/2panel:v1.0.9
```

```bash
docker pull ghcr.io/meimolihan/2panel:latest
```
```bash
docker pull ghcr.io/meimolihan/2panel:v1.0.9
```

## 二进制安装
```bash
bash -c "$(curl -sSL https://raw.githubusercontent.com/meimolihan/2Panel/main/scripts/install.sh)" -p 8080 -d /var/lib/2panel
```

## 二进制卸载
```bash
bash -c "$(curl -sSL https://raw.githubusercontent.com/meimolihan/2Panel/main/scripts/uninstall.sh)" -y --purge
