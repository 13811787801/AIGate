package billing

import (
	"math"

	"aigate/internal/store"
)

type Service struct {
	Store        *store.Store
	FreezeMargin float64
}

func (s *Service) Estimate(model *store.ModelConfig, multiplier float64, maxTokens int) float64 {
	if maxTokens <= 0 {
		maxTokens = model.MaxTokens
	}
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	var cost float64
	switch model.BillingMode {
	case "REQUEST":
		if model.PricePerRequest.Valid {
			cost = model.PricePerRequest.Float64
		}
	default:
		// 按「可能的最大输出」估算 + 少量输入预算
		in := model.InputPricePer1K * 0.5 // 假设约 500 input tokens
		out := model.OutputPricePer1K * (float64(maxTokens) / 1000.0)
		cost = in + out
	}
	cost *= multiplier
	margin := s.FreezeMargin
	if margin <= 0 {
		margin = 1.2
	}
	est := cost * margin
	if est < 0.000001 && (model.InputPricePer1K > 0 || model.OutputPricePer1K > 0 || (model.PricePerRequest.Valid && model.PricePerRequest.Float64 > 0)) {
		est = 0.000001
	}
	// 单价全 0（本地免费模型）时冻结 0
	return round6(est)
}

func (s *Service) Actual(model *store.ModelConfig, multiplier float64, inTokens, outTokens int) float64 {
	var cost float64
	switch model.BillingMode {
	case "REQUEST":
		if model.PricePerRequest.Valid {
			cost = model.PricePerRequest.Float64
		}
	default:
		cost = model.InputPricePer1K*(float64(inTokens)/1000.0) + model.OutputPricePer1K*(float64(outTokens)/1000.0)
	}
	return round6(cost * multiplier)
}

func (s *Service) Freeze(userID, apiKeyID int64, amount float64, traceID string) error {
	if amount <= 0 {
		return nil
	}
	return s.Store.FreezeBalance(userID, amount, apiKeyID, traceID)
}

func (s *Service) Settle(userID, apiKeyID int64, frozen, actual float64, traceID string) error {
	if frozen <= 0 && actual <= 0 {
		return nil
	}
	if frozen <= 0 {
		// 无冻结时直接记一笔（免费模型）
		if actual <= 0 {
			return nil
		}
		return s.Store.SettleBalance(userID, 0, actual, apiKeyID, traceID)
	}
	return s.Store.SettleBalance(userID, frozen, actual, apiKeyID, traceID)
}

func (s *Service) Unfreeze(userID, apiKeyID int64, amount float64, traceID string) error {
	if amount <= 0 {
		return nil
	}
	return s.Store.UnfreezeBalance(userID, amount, apiKeyID, traceID)
}

func round6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}
