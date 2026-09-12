package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"

	"github.com/suprelory/redact-gateway/pkg/types"
)

// 英文字母二元组频率表（基于 CosyRedactGateway 的 BIGRAM_COST）
// 转换为 Go map 结构，使用交叉熵成本（bits）
var bigramCost = map[rune]map[rune]float64{
	'^': {'a': 4.2164, 'b': 4.1407, 'c': 3.7977, 'd': 4.542, 'e': 4.8087, 'f': 4.696, 'g': 5.0298, 'h': 4.7512, 'i': 4.542, 'j': 6.1639, 'k': 6.2409, 'l': 4.4028, 'm': 4.1043, 'n': 4.9961, 'o': 5.0644, 'p': 3.6073, 'q': 7.2213, 'r': 4.7797, 's': 3.3406, 't': 3.9672, 'u': 5.3754, 'v': 6.2409, 'w': 4.338, 'x': 7.5727, 'y': 6.7028, 'z': 7.7869, '0': 12.4307, '$': 12.4307},
	'a': {'b': 5.1033, 'c': 4.8223, 'd': 4.9283, 'l': 3.5288, 'm': 4.1427, 'n': 2.5524, 'r': 3.3525, 's': 3.7296, 't': 3.4477, 'u': 5.6201, 'v': 7.555, 'w': 6.9986, 'y': 6.1506, '$': 2.9525},
	// 简化版本：只保留常用组合，完整版本过长
	'e': {'d': 4.6956, 'l': 4.2353, 'n': 3.2109, 'r': 2.5996, 's': 3.3111, 't': 4.3834, '$': 2.4451},
	'i': {'c': 3.9359, 'n': 2.0263, 'o': 4.3483, 's': 3.6962, 't': 4.0372, '$': 4.6354},
	'o': {'f': 6.5342, 'n': 2.4198, 'r': 3.0031, 't': 4.0385, 'u': 3.527, 'w': 4.3077, '$': 3.5856},
	'u': {'l': 3.9961, 'n': 2.7383, 'r': 3.3364, 's': 3.0493, 't': 3.9148, '$': 4.2717},
}

// 熵阈值表（根据长度确定阈值）
var entropyThresholds = [][]float64{
	{9, 5.424}, {10, 5.3667}, {11, 5.3423}, {12, 5.282}, {13, 5.2565},
	{16, 5.1799}, {20, 5.0612}, {24, 4.9833}, {32, 4.8907}, {40, 4.8327},
	{48, 4.7662}, {56, 4.7277}, {64, 4.7052}, {80, 4.6549}, {96, 4.6248},
	{112, 4.5948}, {128, 4.566},
}

// GetEntropyThreshold 根据长度返回交叉熵阈值（长度感知）
func GetEntropyThreshold(length int) float64 {
	if length <= 8 {
		return math.Inf(1)
	}

	lastIdx := len(entropyThresholds) - 1
	if length >= int(entropyThresholds[lastIdx][0]) {
		return entropyThresholds[lastIdx][1]
	}

	for i := 0; i < lastIdx; i++ {
		l1, h1 := entropyThresholds[i][0], entropyThresholds[i][1]
		l2, h2 := entropyThresholds[i+1][0], entropyThresholds[i+1][1]

		if float64(length) >= l1 && float64(length) <= l2 {
			t := (float64(length) - l1) / (l2 - l1)
			return h1 + (h2-h1)*t
		}
	}

	return entropyThresholds[0][1]
}

// CalculateCrossEntropy 计算字符串的交叉熵（基于英文二元组频率）
func CalculateCrossEntropy(text string) float64 {
	if len(text) < 2 {
		return 0
	}

	normalized := strings.ToLower(text)
	normalized = regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(normalized, "")

	if len(normalized) < 2 {
		return 0
	}

	var bits float64
	prev := '^'

	for _, ch := range normalized {
		row, ok := bigramCost[prev]
		if !ok {
			row = bigramCost['^']
		}

		cost, ok := row[ch]
		if !ok {
			cost = 12.0 // 默认高成本
		}

		bits += cost
		prev = ch
	}

	// 添加结束符成本
	if row, ok := bigramCost[prev]; ok {
		if endCost, ok := row['$']; ok {
			bits += endCost
		} else {
			bits += 12.0
		}
	}

	return bits / float64(len(normalized)+1)
}

// CalculateShannonEntropy 计算 Shannon 熵（字符级）
func CalculateShannonEntropy(text string) float64 {
	if len(text) == 0 {
		return 0
	}

	freq := make(map[rune]int)
	for _, ch := range text {
		freq[ch]++
	}

	var entropy float64
	length := float64(len(text))

	for _, count := range freq {
		p := float64(count) / length
		entropy -= p * math.Log2(p)
	}

	return entropy
}

// IsHighEntropy 判断是否为高熵字符串
func IsHighEntropy(text string) bool {
	if len(text) <= 8 {
		return false
	}

	// 必须包含字母和数字
	hasLetter := false
	hasDigit := false

	for _, ch := range text {
		if unicode.IsLetter(ch) {
			hasLetter = true
		}
		if unicode.IsDigit(ch) {
			hasDigit = true
		}
	}

	if !hasLetter || !hasDigit {
		return false
	}

	// 纯数字跳过
	if regexp.MustCompile(`^\d+$`).MatchString(text) {
		return false
	}

	// Shannon 熵检查（排除低熵重复字符串）
	shannon := CalculateShannonEntropy(text)
	minShannon := math.Min(2.5, math.Log2(float64(len(text)))*0.72)

	if shannon < minShannon {
		return false
	}

	// 交叉熵检查
	crossEntropy := CalculateCrossEntropy(text)
	threshold := GetEntropyThreshold(len(text))

	return crossEntropy > threshold
}

// FindHighEntropyBlocks 在文本中查找所有高熵块
func FindHighEntropyBlocks(text string) []types.Match {
	var matches []types.Match

	// 按字母数字块分割
	re := regexp.MustCompile(`[A-Za-z0-9]+`)
	blocks := re.FindAllStringIndex(text, -1)

	for _, loc := range blocks {
		block := text[loc[0]:loc[1]]

		if len(block) >= 9 && len(block) <= 128 {
			if IsHighEntropy(block) {
				matches = append(matches, types.Match{
					Text:     block,
					Start:    loc[0],
					End:      loc[1],
					RuleID:   "high-entropy",
					RuleType: "HIGH_ENTROPY",
					Priority: 70,
				})
			}
		}
	}

	return matches
}

// GeneratePlaceholder 生成占位符（使用 SHA256 哈希）
// 参考 CosyRedactGateway 格式: {{Redact:sha256}}
func GeneratePlaceholder(plaintext, runtimeSalt string) string {
	h := sha256.New()
	h.Write([]byte(plaintext))
	h.Write([]byte(runtimeSalt))
	hash := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("{{Redact:%s}}", hash)
}

// PlaceholderPattern 占位符正则模式
var PlaceholderPattern = regexp.MustCompile(`\{\{Redact:[a-f0-9]{64}\}\}`)

// ExtractPlaceholders 从文本中提取所有占位符
func ExtractPlaceholders(text string) []string {
	return PlaceholderPattern.FindAllString(text, -1)
}
