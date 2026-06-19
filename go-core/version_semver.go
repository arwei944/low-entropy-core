// Package core — 通用版本管理模块 (v0.8.0)
//
// version_semver.go: SemVer 2.0.0 解析与自动推断
//
// 功能：
//   - 完整 SemVer 2.0.0 解析（支持 v 前缀、预发布、构建元数据）
//   - 版本号比较、排序
//   - 根据 Conventional Commits 自动推断下一版本号

package core

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// semverRegex 匹配 SemVer 2.0.0 格式。
// 示例: 0.7.0, v1.2.3, 0.7.0-alpha.1, v1.0.0-beta.2+build.123
var semverRegex = regexp.MustCompile(
	`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+([0-9A-Za-z.-]+))?$`,
)

// ParseSemver 解析语义化版本号字符串。
// 支持格式: "0.7.0", "v0.7.0", "0.7.0-alpha.1", "v1.0.0+build.123"
func ParseSemver(v string) (Semver, error) {
	matches := semverRegex.FindStringSubmatch(strings.TrimSpace(v))
	if matches == nil {
		return Semver{}, fmt.Errorf("invalid semver: %q", v)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return Semver{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: matches[4],
		Build:      matches[5],
	}, nil
}

// String 返回标准 SemVer 字符串（不含 v 前缀）。
func (s Semver) String() string {
	result := fmt.Sprintf("%d.%d.%d", s.Major, s.Minor, s.Patch)
	if s.Prerelease != "" {
		result += "-" + s.Prerelease
	}
	if s.Build != "" {
		result += "+" + s.Build
	}
	return result
}

// TagName 返回 Git tag 格式的版本号（含 v 前缀）。
func (s Semver) TagName() string {
	return "v" + s.String()
}

// Compare 比较两个版本号。
// 返回值: -1 表示 s < other, 0 表示相等, 1 表示 s > other
func (s Semver) Compare(other Semver) int {
	if s.Major != other.Major {
		return compareInt(s.Major, other.Major)
	}
	if s.Minor != other.Minor {
		return compareInt(s.Minor, other.Minor)
	}
	if s.Patch != other.Patch {
		return compareInt(s.Patch, other.Patch)
	}

	// 预发布版本比较：有预发布的 < 无预发布的
	if s.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if s.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if s.Prerelease != "" && other.Prerelease != "" {
		return comparePrerelease(s.Prerelease, other.Prerelease)
	}

	return 0
}

// compareInt 比较两个整数。
func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// comparePrerelease 比较两个预发布标识。
// 按点号分割，逐段比较：纯数字按数值比较，否则按字符串比较。
func comparePrerelease(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		if i >= len(partsA) {
			return -1 // a 更短，a < b
		}
		if i >= len(partsB) {
			return 1 // b 更短，a > b
		}

		// 尝试按数字比较
		numA, errA := strconv.Atoi(partsA[i])
		numB, errB := strconv.Atoi(partsB[i])

		if errA == nil && errB == nil {
			if numA != numB {
				return compareInt(numA, numB)
			}
		} else {
			// 按字符串比较
			if partsA[i] != partsB[i] {
				if partsA[i] < partsB[i] {
					return -1
				}
				return 1
			}
		}
	}
	return 0
}

// IsValidSemver 验证字符串是否为有效的语义版本号。
func IsValidSemver(v string) bool {
	_, err := ParseSemver(v)
	return err == nil
}

// SortSemvers 按版本号降序排列。
func SortSemvers(versions []Semver) []Semver {
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Compare(versions[j]) > 0
	})
	return versions
}

// BumpMajor 返回 MAJOR+1 的新版本号。
func (s Semver) BumpMajor() Semver {
	return Semver{
		Major: s.Major + 1,
		Minor: 0,
		Patch: 0,
	}
}

// BumpMinor 返回 MINOR+1 的新版本号。
func (s Semver) BumpMinor() Semver {
	return Semver{
		Major: s.Major,
		Minor: s.Minor + 1,
		Patch: 0,
	}
}

// BumpPatch 返回 PATCH+1 的新版本号。
func (s Semver) BumpPatch() Semver {
	return Semver{
		Major: s.Major,
		Minor: s.Minor,
		Patch: s.Patch + 1,
	}
}

// BumpPrerelease 将预发布标识递增。
// 如 alpha.1 → alpha.2, beta.1 → beta.2
func (s Semver) BumpPrerelease() Semver {
	result := Semver{Major: s.Major, Minor: s.Minor, Patch: s.Patch}
	if s.Prerelease == "" {
		result.Prerelease = "alpha.1"
		return result
	}

	parts := strings.Split(s.Prerelease, ".")
	if len(parts) > 0 {
		lastIdx := len(parts) - 1
		if num, err := strconv.Atoi(parts[lastIdx]); err == nil {
			parts[lastIdx] = strconv.Itoa(num + 1)
		} else {
			parts = append(parts, "1")
		}
	}
	result.Prerelease = strings.Join(parts, ".")
	return result
}

// IsZero 判断版本号是否为零值。
func (s Semver) IsZero() bool {
	return s.Major == 0 && s.Minor == 0 && s.Patch == 0
}

// ============================================================================
// 版本号自动推断
// ============================================================================

// InferBump 根据提交列表推断版本号增量类型。
// 规则:
//   - 包含 BREAKING CHANGE → MAJOR bump
//   - 包含 feat 类型 → MINOR bump
//   - 其他（fix, docs, etc.）→ PATCH bump
// 在 0.x 阶段：feat 等同于 MINOR bump（可能不兼容）
func InferBump(commits []ConventionalCommit) VersionBump {
	bump := VersionBump{}

	for _, c := range commits {
		if c.BreakingChange {
			bump.Major = true
		}
		switch c.Type {
		case "feat":
			bump.Minor = true
		case "fix":
			bump.Patch = true
		}
	}

	// 优先级: MAJOR > MINOR > PATCH
	if bump.Major {
		return VersionBump{Major: true}
	}
	if bump.Minor {
		return VersionBump{Minor: true}
	}
	if bump.Patch {
		return VersionBump{Patch: true}
	}

	// 默认 PATCH
	return VersionBump{Patch: true}
}

// InferNextVersion 根据提交列表和当前版本号推断下一版本。
func InferNextVersion(commits []ConventionalCommit, current Semver) Semver {
	bump := InferBump(commits)

	switch {
	case bump.Major:
		return current.BumpMajor()
	case bump.Minor:
		// 0.x 阶段: MINOR bump 就是 MAJOR 级别
		if current.Major == 0 {
			return current.BumpMinor()
		}
		return current.BumpMinor()
	case bump.Patch:
		return current.BumpPatch()
	default:
		return current.BumpPatch()
	}
}

// InferNextVersionFromLog 从 git log 字符串推断下一版本。
// 这是一个便捷函数，组合了 ParseCommitsFromLog + InferNextVersion。
func InferNextVersionFromLog(log string, current Semver) (Semver, error) {
	commits, err := ParseCommitsFromLog(log)
	if err != nil {
		return current, fmt.Errorf("parse commits: %w", err)
	}
	if len(commits) == 0 {
		return current, fmt.Errorf("no commits to analyze")
	}
	return InferNextVersion(commits, current), nil
}