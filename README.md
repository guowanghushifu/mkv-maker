# mkv-remux-web

`mkv-remux-web` 是一个用于蓝光盘Remux视频到mkv的Web工具。
- 支持 **BDMV 输入**，并可在 Linux 容器/主机上自动扫描 `.iso` 输入源。
- `ENABLE_ISO_SCAN`（默认：`1`）：是否扫描 `.iso` 输入源；当容器环境无法执行 `mount -o loop,ro` / `umount` 时，建议设为 `0`
- `/bd_input/iso_auto_mount`：程序保留的 ISO 自动挂载目录，扫描时会被忽略，不应放置用户输入
- ISO 支持仅限 Linux；需要容器具备挂载 loop 设备的权限。若不满足这些条件，请先手动挂载 ISO 再使用 BDMV 输入
```bash
mount -o loop your_bluray_file.iso /your/mount/path/your_bluray_name
```
- 必须提供 **BDInfo 文本** 来判断播放列表和音轨、字幕轨道名称。

## Docker运行

服务端使用以下环境变量：

一般来说你只需要填写 `APP_PASSWORD`
- `APP_PASSWORD`（必填）：Web 应用登录密码
- `APP_DATA_DIR`（默认：`/app/data`）：应用日志目录
- `BD_INPUT_DIR`（默认：`/bd_input`）：挂载的 BDMV 源目录
- `REMUX_OUTPUT_DIR`（默认：`/remux`）：remux 输出目录
- `LISTEN_ADDR`（默认：`:8080`）：HTTP 监听地址
- `SESSION_COOKIE_SECURE`（默认：`0`）：是否为登录会话写入 `Secure` Cookie。通过 HTTPS 或反向代理访问时可显式设为 `1`

默认配置允许明文 HTTP 访问；若部署在公网或 HTTPS 反向代理之后，建议显式启用 `SESSION_COOKIE_SECURE=1`。

Docker Compose 示例：

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
    volumes:
      - ./data:/app/data           # 日志目录
      - /dld:/bd_input:rshared     # 蓝光盘存放目录；ISO 文件也可放在此目录下
      - /remux:/remux              # remux输出目录
```

如果容器内无法以 `mount -o loop,ro` 和 `umount` 完成自动挂载，请将 `ENABLE_ISO_SCAN=0`，并改为在宿主机上手动挂载 ISO 后把 BDMV 目录挂入容器。

## Docker构建和运行（本地）

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

