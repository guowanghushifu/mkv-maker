# mkv-remux-web

`mkv-remux-web` 是一个从蓝光盘 Remux 视频到 mkv 的 Web 工具。支持输入 **BDMV 目录** 和 `ISO 光盘`。

# 注意事项
- 必须提供 **BDInfo 文本** 来判断播放列表和音轨、字幕轨道名称。
- 本软件使用了Makemkv来处理蓝光盘，但Makemkv隔段时间就会更新版本，老版本会过期，Beta Key也会过期，我会尽量跟随官方更新
- remux输出目录空间约束：如果处理UHD蓝光盘，建议预留200GB空间；需要先用makemkv生成一个临时中间文件，再二次裁剪为目标文件，过程中需要占用两份空间。

## Docker运行

服务端使用以下环境变量：
- `APP_PASSWORD`（必填）：Web 应用登录密码
- `MAKEMKV_EXPIRE_DATE`（默认：`Dockerfile中指定`）：如果设置为有效的 `YYYY-MM-DD`，且当前系统日期晚于该日期，在执行 `makemkvcon info` / `makemkvcon mkv` 前，临时把系统日期调整到过期日期前30天；命令运行满 3 秒后会恢复正常日期。此环境变量主要是为了解决Makemkv版本和Beta Key过期的问题，如果你不需要这个功能，你可以设置为2099-01-01，这样不会触发调整日期。
- `SESSION_COOKIE_SECURE`（默认：`0`）：是否为登录会话写入 `Secure` Cookie；通过 HTTPS 或反向代理访问时可显式设为 `1`

Docker Compose 示例：BDMV / ISO 通用场景：

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
    cap_add:
      - SYS_TIME                   # 允许容器修改宿主机系统时间
    security_opt:
      - seccomp=unconfined         # 某些 Docker 默认 seccomp 会拦截 date/settimeofday
    volumes:
      - ./data:/app/data           # 日志目录
      - ./config:/config           # MakeMKV 配置目录，包含 /config/settings.conf
      - /dld:/bd_input:rshared     # 蓝光盘目录与 ISO 文件目录
      - /remux:/remux              # remux输出目录
      - /remux_tmp:/remux_tmp      # makemkv的临时工作目录，⚠️ 必须指向空目录
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
