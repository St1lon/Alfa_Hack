package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/st1lon/alfa-hack/internal/domain"
)

// LLMModels — содержимое configs/llm.yaml: allowlist, маршрутизация, цены.
type LLMModels struct {
	FallbackModel string            `yaml:"fallback_model"`
	Models        []LLMModelEntry   `yaml:"models"`
	Routing       map[string]string `yaml:"routing"`
}

// LLMModelEntry — одна разрешённая модель и её цена.
// Цены в копейках за миллион токенов; 0 = провайдер не берёт денег за это направление.
type LLMModelEntry struct {
	ID              string `yaml:"id"`
	Note            string `yaml:"note"`
	PriceInKopPerM  int64  `yaml:"price_in_kop_per_mtok"`
	PriceOutKopPerM int64  `yaml:"price_out_kop_per_mtok"`
}

// LoadLLMModels читает файл allowlist моделей.
func LoadLLMModels(path string) (*LLMModels, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read llm models file: %w", err)
	}

	var models LLMModels
	if err = yaml.Unmarshal(raw, &models); err != nil {
		return nil, fmt.Errorf("unmarshal llm models file: %w", err)
	}
	if len(models.Models) == 0 {
		return nil, fmt.Errorf("llm models file %s: allowlist is empty", path)
	}

	return &models, nil
}

// Domain преобразует запись файла в доменную модель.
func (e LLMModelEntry) Domain() domain.LLMModel {
	return domain.LLMModel{
		ID:              e.ID,
		PriceInKopPerM:  e.PriceInKopPerM,
		PriceOutKopPerM: e.PriceOutKopPerM,
	}
}

// DomainModels возвращает allowlist в доменных типах.
func (m *LLMModels) DomainModels() []domain.LLMModel {
	models := make([]domain.LLMModel, 0, len(m.Models))
	for _, entry := range m.Models {
		models = append(models, entry.Domain())
	}
	return models
}

// DomainRouting возвращает карту «задача → модель» в доменных типах.
func (m *LLMModels) DomainRouting() map[domain.LLMTask]string {
	routing := make(map[domain.LLMTask]string, len(m.Routing))
	for task, model := range m.Routing {
		routing[domain.LLMTask(task)] = model
	}
	return routing
}
