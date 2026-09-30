package objectkey

import (
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// internalObjectMarker 是 v2 object key 中的内部保留段；生成器、解析器与 code 校验共同引用这一处常量。
const internalObjectMarker = ".admin-storage"

// objectKeyVersionSegment 固定在保留段之后，用于区分将来的 key 协议版本。
const objectKeyVersionSegment = "v2"

// ErrInvalid 表示 object key 不是本协议 v2 格式（旧格式、缺段、结构损坏一律按无效请求处理）。
var ErrInvalid = errors.New("object key is invalid")

// Validate lets settings and other consumers share the exact storage key protocol.
func Validate(value string) error {
	_, err := Parse(value)
	return err
}

// codeSegmentPattern 与数据库 CHECK 契约逐字一致：每段以 [a-z0-9] 开头，后续允许 [a-z0-9._-]。
var codeSegmentPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

var (
	randomSegmentPattern    = regexp.MustCompile(`^[0-9a-f]{32}$`)
	extensionPattern        = regexp.MustCompile(`^[a-z0-9]{1,16}$`)
	identifierSegmentDigits = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// ValidCode 执行与 storage_upload_rule_code CHECK 相同的契约：
// trim + lower 后 1..64 ASCII 字节；允许 `/` 分隔；禁止空段、`..`、内部保留段与非法字符。
func ValidCode(value string) bool {
	if len(value) < 1 || len(value) > 64 || value != strings.ToLower(strings.TrimSpace(value)) || strings.Contains(value, "..") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if !codeSegmentPattern.MatchString(segment) || segment == internalObjectMarker {
			return false
		}
	}
	return true
}

// Coordinates 是 v2 object key 自带的物理坐标：平台、规则、COS 配置与物理版本。
type Coordinates struct {
	PlatformID  int64
	RuleID      int64
	CosConfigID int64
	Version     int64
}

// Generate 生成固定格式：
// <code>/.admin-storage/v2/p<platformId>/r<ruleId>/c<configId>/v<version>/YYYY/MM/DD/<32 lowercase hex>.<ext>
func Generate(code string, coordinates Coordinates, extension string, now time.Time) (string, error) {
	if !ValidCode(code) {
		return "", fmt.Errorf("%w: code is invalid", ErrInvalid)
	}
	if coordinates.PlatformID < 1 || coordinates.RuleID < 1 || coordinates.CosConfigID < 1 || coordinates.Version < 1 {
		return "", fmt.Errorf("%w: coordinates are invalid", ErrInvalid)
	}
	extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extension)), ".")
	if !extensionPattern.MatchString(extension) {
		return "", fmt.Errorf("%w: extension is invalid", ErrInvalid)
	}
	random, err := randomObjectKeySegment()
	if err != nil {
		return "", err
	}
	utc := now.UTC()
	return fmt.Sprintf("%s/%s/%s/p%d/r%d/c%d/v%d/%04d/%02d/%02d/%s.%s",
		strings.Trim(code, "/"), internalObjectMarker, objectKeyVersionSegment,
		coordinates.PlatformID, coordinates.RuleID, coordinates.CosConfigID, coordinates.Version,
		utc.Year(), utc.Month(), utc.Day(), random, extension), nil
}

// Parse 严格解析 v2 key：只用 key 中的坐标，不用 code 推断路由。
func Parse(objectKey string) (Coordinates, error) {
	if objectKey == "" || len(objectKey) > 1024 || strings.Contains(objectKey, "..") ||
		strings.ContainsAny(objectKey, "\\\r\n\t") || strings.IndexFunc(objectKey, isControlRune) >= 0 {
		return Coordinates{}, fmt.Errorf("%w: key characters are invalid", ErrInvalid)
	}
	segments := strings.Split(objectKey, "/")
	markerIndex := -1
	for index, segment := range segments {
		if segment == internalObjectMarker {
			if markerIndex >= 0 {
				return Coordinates{}, fmt.Errorf("%w: marker is duplicated", ErrInvalid)
			}
			markerIndex = index
		}
	}
	if markerIndex < 1 || markerIndex+1 >= len(segments) || segments[markerIndex+1] != objectKeyVersionSegment {
		return Coordinates{}, fmt.Errorf("%w: marker or version segment is missing", ErrInvalid)
	}
	code := strings.Join(segments[:markerIndex], "/")
	if !ValidCode(code) {
		return Coordinates{}, fmt.Errorf("%w: code is invalid", ErrInvalid)
	}
	// 期望结构：p<id>/r<id>/c<id>/v<id>/YYYY/MM/DD/<random>.<ext>
	body := segments[markerIndex+2:]
	if len(body) != 8 {
		return Coordinates{}, fmt.Errorf("%w: segment count is invalid", ErrInvalid)
	}
	platformID, err := parseIdentifierSegment(body[0], "p")
	if err != nil {
		return Coordinates{}, err
	}
	ruleID, err := parseIdentifierSegment(body[1], "r")
	if err != nil {
		return Coordinates{}, err
	}
	cosConfigID, err := parseIdentifierSegment(body[2], "c")
	if err != nil {
		return Coordinates{}, err
	}
	version, err := parseIdentifierSegment(body[3], "v")
	if err != nil {
		return Coordinates{}, err
	}
	if !yearSegmentPattern.MatchString(body[4]) || !digitsSegmentPattern.MatchString(body[5]) || !digitsSegmentPattern.MatchString(body[6]) {
		return Coordinates{}, fmt.Errorf("%w: date segments are invalid", ErrInvalid)
	}
	if _, err := time.Parse("2006/01/02", body[4]+"/"+body[5]+"/"+body[6]); err != nil {
		return Coordinates{}, fmt.Errorf("%w: date is invalid", ErrInvalid)
	}
	randomAndExtension := strings.Split(body[7], ".")
	if len(randomAndExtension) != 2 || !randomSegmentPattern.MatchString(randomAndExtension[0]) || !extensionPattern.MatchString(randomAndExtension[1]) {
		return Coordinates{}, fmt.Errorf("%w: object name is invalid", ErrInvalid)
	}
	return Coordinates{PlatformID: platformID, RuleID: ruleID, CosConfigID: cosConfigID, Version: version}, nil
}

var (
	digitsSegmentPattern = regexp.MustCompile(`^[0-9]{2}$`)
	yearSegmentPattern   = regexp.MustCompile(`^[0-9]{4}$`)
)

func parseIdentifierSegment(segment, prefix string) (int64, error) {
	if !strings.HasPrefix(segment, prefix) {
		return 0, fmt.Errorf("%w: %s segment is missing", ErrInvalid, prefix)
	}
	digits := strings.TrimPrefix(segment, prefix)
	if !identifierSegmentDigits.MatchString(digits) {
		return 0, fmt.Errorf("%w: %s identifier is invalid", ErrInvalid, prefix)
	}
	value, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("%w: %s identifier is out of range", ErrInvalid, prefix)
	}
	return value, nil
}

func isControlRune(r rune) bool { return r < 0x20 || r == 0x7f }

func randomObjectKeySegment() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate object key random segment: %w", err)
	}
	return fmt.Sprintf("%x", buffer), nil
}
