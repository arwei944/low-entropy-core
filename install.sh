#!/bin/bash
# arch-cli 一键安装脚本 (Linux/macOS)
# 使用方法:
#   curl -fsSL https://raw.githubusercontent.com/USERNAME/REPO/main/install.sh | bash
# 或
#   wget -qO- https://raw.githubusercontent.com/USERNAME/REPO/main/install.sh | bash

set -e

# 颜色输出
COLOR_CYAN='\033[0;36m'
COLOR_GREEN='\033[0;32m'
COLOR_YELLOW='\033[1;33m'
COLOR_RED='\033[0;31m'
COLOR_NC='\033[0m' # No Color

# 默认配置
VERSION="latest"
INSTALL_DIR="$HOME/.local/bin"
REPO_OWNER="USERNAME"
REPO_NAME="REPO"
API_URL="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest"

# 解析参数
while [ "$#" -gt 0 ]; do
    case "$1" in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -d|--dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# 检测系统
detect_os() {
    case "$(uname -s)" in
        Linux*)
            echo "linux"
            ;;
        Darwin*)
            echo "darwin"
            ;;
        *)
            echo "unknown"
            ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64*|amd64*)
            echo "amd64"
            ;;
        arm64*|aarch64*)
            echo "arm64"
            ;;
        *)
            echo "unknown"
            ;;
    esac
}

OS=$(detect_os)
ARCH=$(detect_arch)
EXT=""

if [ "$OS" = "unknown" ] || [ "$ARCH" = "unknown" ]; then
    echo -e "${COLOR_RED}✗ 不支持的系统/架构: $(uname -s)/$(uname -m)${COLOR_NC}"
    exit 1
fi

echo -e "${COLOR_CYAN}========================================${COLOR_NC}"
echo -e "${COLOR_CYAN}  arch-cli 一键安装${COLOR_NC}"
echo -e "${COLOR_CYAN}========================================${COLOR_NC}"
echo ""
echo -e "系统: ${COLOR_GREEN}$OS/$ARCH${COLOR_NC}"
echo ""

# 1. 创建安装目录
echo -e "${COLOR_YELLOW}[1/5] 创建安装目录...${COLOR_NC}"
mkdir -p "$INSTALL_DIR"
echo -e "${COLOR_GREEN}✓ $INSTALL_DIR${COLOR_NC}"
echo ""

# 2. 获取版本信息
echo -e "${COLOR_YELLOW}[2/5] 获取版本信息...${COLOR_NC}"

# 尝试从 GitHub 获取最新版本
if command -v curl &> /dev/null; then
    HTTP_CLIENT="curl -fsSL"
elif command -v wget &> /dev/null; then
    HTTP_CLIENT="wget -qO-"
else
    echo -e "${COLOR_RED}✗ 需要 curl 或 wget${COLOR_NC}"
    exit 1
fi

# 检查是否有本地测试文件
LOCAL_TEST=0
if [ -f "./arch-cli" ]; then
    echo -e "${COLOR_YELLOW}发现本地文件，使用测试模式${COLOR_NC}"
    LOCAL_TEST=1
    INSTALL_VERSION="v1.0.0"
else
    # 尝试获取 GitHub 版本
    set +e
    RESPONSE=$($HTTP_CLIENT "$API_URL" 2>/dev/null)
    set -e

    if [ -n "$RESPONSE" ]; then
        LATEST_VERSION=$(echo "$RESPONSE" | grep -o '"tag_name": "[^"]*"' | head -1 | sed 's/"tag_name": "//;s/"//')
        if [ "$VERSION" = "latest" ]; then
            INSTALL_VERSION="$LATEST_VERSION"
        else
            INSTALL_VERSION="$VERSION"
        fi
        echo -e "${COLOR_GREEN}✓ 最新版本: $LATEST_VERSION${COLOR_NC}"
    else
        echo -e "${COLOR_YELLOW}无法访问 GitHub，使用本地编译${COLOR_NC}"
        if command -v go &> /dev/null; then
            echo -e "${COLOR_YELLOW}正在编译...${COLOR_NC}"
            go build -tags lecore_tier4 -o arch-cli ./cmd/arch-cli
            if [ -f "./arch-cli" ]; then
                LOCAL_TEST=1
                INSTALL_VERSION="v1.0.0"
            else
                echo -e "${COLOR_RED}✗ 编译失败${COLOR_NC}"
                exit 1
            fi
        else
            echo -e "${COLOR_RED}✗ 请先发布一个 Release 或安装 Go${COLOR_NC}"
            exit 1
        fi
    fi
fi
echo ""

# 3. 下载文件
echo -e "${COLOR_YELLOW}[3/5] 下载 arch-cli...${COLOR_NC}"
TARGET_NAME="arch-cli-${INSTALL_VERSION}-${OS}-${ARCH}${EXT}"
INSTALL_PATH="$INSTALL_DIR/arch-cli"

if [ "$LOCAL_TEST" -eq 1 ]; then
    # 使用本地文件
    cp "./arch-cli" "$INSTALL_PATH"
    chmod +x "$INSTALL_PATH"
    echo -e "${COLOR_GREEN}✓ 已复制本地文件${COLOR_NC}"
else
    # 从 GitHub 下载
    ASSET_URL=$(echo "$RESPONSE" | grep -o '"browser_download_url": "[^"]*'"${TARGET_NAME}"'"' | head -1 | sed 's/"browser_download_url": "//;s/"//')

    if [ -z "$ASSET_URL" ]; then
        echo -e "${COLOR_RED}✗ 未找到匹配的资产: $TARGET_NAME${COLOR_NC}"
        echo "可用资产:"
        echo "$RESPONSE" | grep -o '"name": "[^"]*"' | sed 's/"name": "//;s/"//' | while read -r name; do
            echo "  - $name"
        done
        exit 1
    fi

    echo "从 $ASSET_URL 下载..."

    if command -v curl &> /dev/null; then
        curl -fSL "$ASSET_URL" -o "$INSTALL_PATH"
    else
        wget -q "$ASSET_URL" -O "$INSTALL_PATH"
    fi

    chmod +x "$INSTALL_PATH"
    echo -e "${COLOR_GREEN}✓ 已下载${COLOR_NC}"
fi
echo ""

# 4. 验证安装
echo -e "${COLOR_YELLOW}[4/5] 验证安装...${COLOR_NC}"
if [ -f "$INSTALL_PATH" ]; then
    FILE_SIZE=$(wc -c < "$INSTALL_PATH" 2>/dev/null || ls -l "$INSTALL_PATH" | awk '{print $5}')
    SIZE_KB=$(echo "scale=2; $FILE_SIZE / 1024" | bc 2>/dev/null || echo "0.00")
    echo -e "${COLOR_GREEN}✓ 文件: $INSTALL_PATH (${SIZE_KB} KB)${COLOR_NC}"
else
    echo -e "${COLOR_RED}✗ 安装失败，文件不存在${COLOR_NC}"
    exit 1
fi
echo ""

# 5. 检查 PATH
echo -e "${COLOR_YELLOW}[5/5] 检查环境变量...${COLOR_NC}"
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo -e "${COLOR_YELLOW}建议将 $INSTALL_DIR 添加到 PATH${COLOR_NC}"
    echo ""
    echo "在 ~/.bashrc 或 ~/.zshrc 中添加:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    echo ""
    echo "然后运行: source ~/.bashrc (或 ~/.zshrc)"
else
    echo -e "${COLOR_GREEN}✓ 已在 PATH 中${COLOR_NC}"
fi
echo ""

# 完成
echo -e "${COLOR_CYAN}========================================${COLOR_NC}"
echo -e "${COLOR_CYAN}  安装完成! ($INSTALL_VERSION)${COLOR_NC}"
echo -e "${COLOR_CYAN}========================================${COLOR_NC}"
echo ""
echo "下一步:"
echo -e "  ${COLOR_YELLOW}1. 刷新环境变量 (如果需要)${COLOR_NC}"
echo -e "  ${COLOR_YELLOW}2. 运行: arch-cli version${COLOR_NC}"
echo -e "  ${COLOR_YELLOW}3. 运行: arch-cli help${COLOR_NC}"
echo ""

# 测试运行
echo "测试命令:"
"$INSTALL_PATH" version 2>/dev/null || echo "  请刷新环境变量后测试"
echo ""

