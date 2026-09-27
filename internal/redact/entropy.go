package redact

import (
	_ "embed"
	"encoding/json"
	"math"
)

// Model data and threshold calibration are derived from Cosy Redact Gateway.
// This Go implementation uses fixed tables; attribution is in third_party/.
//
//go:embed data/entropy-model.json
var entropyModelJSON []byte

var entropyModel = loadEntropyModel()

type characterModel struct {
	costs      [128][128]float64
	thresholds [][2]float64
}

func loadEntropyModel() characterModel {
	var data struct {
		Costs      map[string]map[string]float64 `json:"costs"`
		Thresholds [][2]float64                  `json:"thresholds"`
	}
	if err := json.Unmarshal(entropyModelJSON, &data); err != nil {
		panic(err)
	}
	model := characterModel{thresholds: data.Thresholds}
	for row := range model.costs {
		for column := range model.costs[row] {
			model.costs[row][column] = 12
		}
	}
	for previous, row := range data.Costs {
		for current, cost := range row {
			model.costs[previous[0]][current[0]] = cost
		}
	}
	return model
}

func entropyThreshold(length int) float64 {
	if length <= 8 {
		return math.Inf(1)
	}
	anchors := entropyModel.thresholds
	for index := 1; index < len(anchors); index++ {
		left, right := anchors[index-1], anchors[index]
		if float64(length) <= right[0] {
			return left[1] + (right[1]-left[1])*(float64(length)-left[0])/(right[0]-left[0])
		}
	}
	return anchors[len(anchors)-1][1]
}

func shannonEntropy(value string) float64 {
	var counts [256]int
	for index := 0; index < len(value); index++ {
		counts[value[index]]++
	}
	return entropyOfCounts(counts, len(value))
}

func entropyOfCounts(counts [256]int, length int) float64 {
	entropy := 0.0
	for _, count := range counts {
		if count > 0 {
			probability := float64(count) / float64(length)
			entropy -= probability * math.Log2(probability)
		}
	}
	return entropy
}

func isHighEntropy(value string) bool {
	if len(value) <= 8 {
		return false
	}
	var counts [256]int
	previous, bits, allDigits := byte('^'), 0.0, true
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character >= 'A' && character <= 'Z' {
			character += 'a' - 'A'
		}
		if character >= 'a' && character <= 'z' {
			allDigits = false
		} else if !isASCIIDigit(character) {
			return false
		}
		counts[character]++
		bits += entropyModel.costs[previous][character]
		previous = character
	}
	if allDigits || entropyOfCounts(counts, len(value)) < math.Min(2.5, math.Log2(float64(len(value)))*0.72) {
		return false
	}
	bits += entropyModel.costs[previous]['$']
	return bits/float64(len(value)+1) > entropyThreshold(len(value))
}
