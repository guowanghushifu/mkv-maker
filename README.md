# mkv-remux-web

`mkv-remux-web` 是一个从蓝光盘Remux视频到mkv的Web工具。支持输入**BDMV目录**，并可在 Linux 容器/主机上按需扫描 `ISO光盘` 。

# 注意事项
- `/bd_input/iso_auto_mount`：程序保留的 ISO 自动挂载目录，扫描时会被忽略，不应放置蓝光盘
- ISO 支持仅限 Linux；启用自动扫描时需要容器具备挂载 loop 设备的权限。若不满足这些条件，请保持 `ENABLE_ISO_SCAN=0`，手动挂载 ISO
```bash
mount -o loop your_bluray_file.iso /your/mount/path/your_bluray_name
```
- 必须提供 **BDInfo 文本** 来判断播放列表和音轨、字幕轨道名称。

## Docker运行

服务端使用以下环境变量：
- `APP_PASSWORD`（必填）：Web 应用登录密码
- `ENABLE_ISO_SCAN`（默认：`0`）：是否扫描 `.iso` 输入源；仅在需要容器内自动挂载 ISO 时设为 `1`
- `SESSION_COOKIE_SECURE`（默认：`0`）：是否为登录会话写入 `Secure` Cookie；通过 HTTPS 或反向代理访问时可显式设为 `1`
- MakeMKV 配置文件保存在 `/config/settings.conf`；建议把宿主机目录挂载到 `/config` 以便持久化和手工修改
- `app_Key` 由容器在启动时自动刷新，并且每天凌晨 1 点通过 cron 自动更新；如需长期自定义 key，请留意该行为会覆盖不同的现有 `app_Key`

Docker Compose 示例：BDMV 场景（不扫描 ISO）：

```yaml
services:
  mkv-remux-web:
    image: guowanghushifu/mkv-remux-web:latest
    container_name: mkv-remux-web
    restart: unless-stopped
    ports:
      - "38080:8080"
    environment:
      APP_PASSWORD: "你的登录密码"
    volumes:
      - ./data:/app/data           # 日志目录
      - ./config:/config           # MakeMKV 配置目录，包含 /config/settings.conf
      - /dld:/bd_input:rshared     # 蓝光盘存放目录；如需使用 ISO，请先在宿主机手动挂载
      - /remux:/remux              # remux输出目录
```

Docker Compose 示例：BDMV + ISO 自动扫描场景（需要额外权限）：

```yaml
services:
  mkv-remux-web:
    image: guowanghushifu/mkv-remux-web:latest
    container_name: mkv-remux-web
    restart: unless-stopped
    ports:
      - "38080:8080"
    environment:
      APP_PASSWORD: "你的登录密码"
      ENABLE_ISO_SCAN: "1"
    user: "0:0"
    cap_add:
      - SYS_ADMIN
    security_opt:
      - seccomp:unconfined
      - apparmor:unconfined
    devices:
      - /dev/loop-control:/dev/loop-control
      - /dev/loop0:/dev/loop0
      - /dev/loop1:/dev/loop1
      - /dev/loop2:/dev/loop2
      - /dev/loop3:/dev/loop3
      - /dev/loop4:/dev/loop4
      - /dev/loop5:/dev/loop5
    volumes:
      - ./data:/app/data           # 日志目录
      - ./config:/config           # MakeMKV 配置目录，包含 /config/settings.conf
      - /dld:/bd_input:rshared     # 蓝光盘与 ISO 文件目录；容器会自动扫描并挂载 ISO
      - /remux:/remux              # remux输出目录
```

## Docker构建和运行（本地测试使用，普通用户忽略）

构建：

```bash
./scripts/docker-build.sh
```

可选：自定义镜像标签：

```bash
IMAGE_TAG=mkv-remux-web:test ./scripts/docker-build.sh
```

可选：本地构建控制项：

- `NO_CACHE=1`：禁用 Docker 层缓存
- `PLATFORMS=linux/amd64,linux/arm64`：请求使用 Buildx 进行多架构构建
- `PUSH=1`：将生成的镜像推送出去，而不是加载到本地（需要带仓库前缀的 `IMAGE_TAG`）

示例：

```bash
./scripts/docker-build.sh
NO_CACHE=1 ./scripts/docker-build.sh
PLATFORMS=linux/amd64 ./scripts/docker-build.sh
PLATFORMS=linux/amd64,linux/arm64 PUSH=1 IMAGE_TAG=<registry>/<image>:test ./scripts/docker-build.sh
```

运行：

```bash
APP_PASSWORD=change-me ./scripts/docker-run.sh
```

默认情况下，`scripts/docker-run.sh` 会把 `CONFIG_HOST_DIR` 设为 `$PWD/.config`；上面的 Compose 示例则使用 `./config` 挂载到 `/config`。

可选：自定义宿主机配置目录：

```bash
APP_PASSWORD=change-me CONFIG_HOST_DIR=$PWD/my-config ./scripts/docker-run.sh
```

容器首次启动时，如果挂载目录中没有 `settings.conf`，会自动用镜像内默认模板初始化 `/config/settings.conf`。
