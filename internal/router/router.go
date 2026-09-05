package router

import (
	"fmt"
	"math/rand"

	"aigate/internal/store"
)

type Service struct {
	Store *store.Store
}

func (s *Service) Select(modelID int64) (store.ModelChannel, error) {
	list, err := s.Store.ListModelChannels(modelID)
	if err != nil {
		return store.ModelChannel{}, err
	}
	if len(list) == 0 {
		return store.ModelChannel{}, fmt.Errorf("no healthy channel for model")
	}
	total := 0
	for _, mc := range list {
		w := mc.Weight
		if w <= 0 {
			w = mc.Channel.Weight
		}
		if w <= 0 {
			w = 1
		}
		total += w
	}
	r := rand.Intn(total)
	acc := 0
	for _, mc := range list {
		w := mc.Weight
		if w <= 0 {
			w = mc.Channel.Weight
		}
		if w <= 0 {
			w = 1
		}
		acc += w
		if r < acc {
			return mc, nil
		}
	}
	return list[0], nil
}

// SelectAllOrdered 返回按权重尝试的渠道列表（用于故障转移）
func (s *Service) SelectAllOrdered(modelID int64) ([]store.ModelChannel, error) {
	return s.Store.ListModelChannels(modelID)
}
