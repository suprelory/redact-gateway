package engine

import (
	"bufio"
	"bytes"
	"io"
	"strings"

	"github.com/suprelory/redact-gateway/pkg/types"
)

// RestoreOptions 还原选项
type RestoreOptions struct {
	MappingTable map[string]string // placeholder -> plaintext
}

// Restore 执行还原
func Restore(text string, opts RestoreOptions) *types.RestoreResult {
	if len(opts.MappingTable) == 0 {
		return &types.RestoreResult{
			RestoredText: text,
			RestoreCount: 0,
			SkippedCount: 0,
		}
	}

	restoredText := text
	restoreCount := 0
	skippedCount := 0

	// 提取所有占位符
	placeholders := ExtractPlaceholders(text)

	for _, ph := range placeholders {
		if plaintext, ok := opts.MappingTable[ph]; ok {
			restoredText = strings.ReplaceAll(restoredText, ph, plaintext)
			restoreCount++
		} else {
			skippedCount++
		}
	}

	return &types.RestoreResult{
		RestoredText: restoredText,
		RestoreCount: restoreCount,
		SkippedCount: skippedCount,
	}
}

// StreamRestore SSE 流式还原
type StreamRestore struct {
	mappingTable map[string]string
	buffer       *bytes.Buffer
	windowSize   int
	restoreCount int
	skippedCount int
}

// NewStreamRestore 创建流式还原器
func NewStreamRestore(mappingTable map[string]string) *StreamRestore {
	return &StreamRestore{
		mappingTable: mappingTable,
		buffer:       &bytes.Buffer{},
		windowSize:   256, // 滑动窗口大小
	}
}

// Write 写入数据并尝试还原
func (sr *StreamRestore) Write(chunk []byte) ([]byte, error) {
	// 追加到缓冲区
	sr.buffer.Write(chunk)

	// 如果缓冲区过大，强制刷新前面的部分
	if sr.buffer.Len() > sr.windowSize*2 {
		return sr.flush(sr.windowSize)
	}

	// 尝试还原缓冲区中的占位符
	return sr.tryRestore()
}

// Flush 刷新剩余缓冲区
func (sr *StreamRestore) Flush() ([]byte, error) {
	return sr.flush(0)
}

// flush 刷新缓冲区（保留最后 keepSize 字节）
func (sr *StreamRestore) flush(keepSize int) ([]byte, error) {
	bufLen := sr.buffer.Len()
	if bufLen <= keepSize {
		return nil, nil
	}

	flushSize := bufLen - keepSize
	data := sr.buffer.Next(flushSize)

	// 还原刷新的数据
	restored := sr.restoreChunk(data)
	return restored, nil
}

// tryRestore 尝试还原缓冲区内容
func (sr *StreamRestore) tryRestore() ([]byte, error) {
	content := sr.buffer.String()

	// 查找完整的占位符
	matches := PlaceholderPattern.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		// 没有完整占位符，保留可能的不完整占位符
		if sr.buffer.Len() > sr.windowSize {
			return sr.flush(80) // 保留 80 字节（占位符长度 75）
		}
		return nil, nil
	}

	// 找到最后一个完整占位符的结束位置
	lastEnd := matches[len(matches)-1][1]

	// 刷新到最后一个占位符之后
	data := []byte(content[:lastEnd])
	sr.buffer.Next(lastEnd)

	// 还原数据
	restored := sr.restoreChunk(data)
	return restored, nil
}

// restoreChunk 还原数据块
func (sr *StreamRestore) restoreChunk(data []byte) []byte {
	text := string(data)

	placeholders := PlaceholderPattern.FindAllString(text, -1)
	for _, ph := range placeholders {
		if plaintext, ok := sr.mappingTable[ph]; ok {
			text = strings.ReplaceAll(text, ph, plaintext)
			sr.restoreCount++
		} else {
			sr.skippedCount++
		}
	}

	return []byte(text)
}

// GetStats 获取统计信息
func (sr *StreamRestore) GetStats() (restoreCount, skippedCount int) {
	return sr.restoreCount, sr.skippedCount
}

// RestoreSSEStream 还原 SSE 流
func RestoreSSEStream(reader io.Reader, mappingTable map[string]string) (io.Reader, *StreamRestore) {
	pr, pw := io.Pipe()
	sr := NewStreamRestore(mappingTable)

	go func() {
		defer pw.Close()

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Bytes()

			// 还原当前行
			restored, err := sr.Write(line)
			if err != nil {
				return
			}

			if len(restored) > 0 {
				pw.Write(restored)
			}

			// 写入换行符
			pw.Write([]byte("\n"))
		}

		// 刷新剩余缓冲区
		remaining, _ := sr.Flush()
		if len(remaining) > 0 {
			pw.Write(remaining)
		}
	}()

	return pr, sr
}
