#!/bin/bash
# Low-Entropy Core — Pre-Commit 架构约束检查
# 在 git commit 前自动检查架构违规，违规数 > 0 则阻止提交

ARCH_MANAGER_URL="http://localhost:8090"
VIOLATIONS_URL="$ARCH_MANAGER_URL/api/violations"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "============================================"
echo " Low-Entropy Core — 架构约束检查"
echo "============================================"

# 检查 arch-manager 是否在运行
if ! curl -s --max-time 2 "$ARCH_MANAGER_URL/api/health-score" > /dev/null 2>&1; then
    echo -e "${YELLOW}arch-manager 未运行，正在启动...${NC}"
    
    # 编译并启动
    cd "$(git rev-parse --show-toplevel)" || exit 1
    go build -tags lecore_tier4 -o arch-manager.exe ./cmd/arch-manager/ 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${RED}编译失败！请修复编译错误后再提交。${NC}"
        exit 1
    fi
    
    ./arch-manager.exe &
    ARCH_PID=$!
    
    # 等待服务就绪
    for i in $(seq 1 15); do
        if curl -s --max-time 2 "$ARCH_MANAGER_URL/api/health-score" > /dev/null 2>&1; then
            echo -e "${GREEN}arch-manager 已就绪${NC}"
            break
        fi
        sleep 1
    done
fi

# 检查违规
echo "正在检查架构违规..."
VIOLATIONS=$(curl -s --max-time 5 "$VIOLATIONS_URL")

# 解析违规数量
VIOLATION_COUNT=$(echo "$VIOLATIONS" | grep -o '"type"' | wc -l)

if [ "$VIOLATION_COUNT" -gt 0 ]; then
    echo ""
    echo -e "${RED}============================================"
    echo -e " 发现 ${VIOLATION_COUNT} 条架构违规！"
    echo -e "============================================${NC}"
    echo ""
    echo "$VIOLATIONS"
    echo ""
    echo "违规详情也可在仪表盘查看: $ARCH_MANAGER_URL/arch-manager.html"
    echo ""
    echo -e "${RED}提交被阻止。请修复所有违规后重试。${NC}"
    echo -e "${YELLOW}提示: 使用 git commit --no-verify 可跳过检查（不推荐）${NC}"
    
    # 清理
    if [ -n "$ARCH_PID" ]; then
        kill $ARCH_PID 2>/dev/null
    fi
    exit 1
fi

echo -e "${GREEN}架构检查通过！违规数: 0${NC}"

# 检查健康评分
HEALTH=$(curl -s --max-time 5 "$ARCH_MANAGER_URL/api/health-score")
SCORE=$(echo "$HEALTH" | grep -o '"overall":[0-9.]*' | head -1 | cut -d: -f2)

if [ -n "$SCORE" ] && [ "$(echo "$SCORE < 60" | bc 2>/dev/null)" = "1" ]; then
    echo -e "${YELLOW}警告: 健康评分 ${SCORE} 低于 60，建议检查后再提交${NC}"
fi

# 清理
if [ -n "$ARCH_PID" ]; then
    kill $ARCH_PID 2>/dev/null
fi

echo -e "${GREEN}预提交检查全部通过！${NC}"
exit 0