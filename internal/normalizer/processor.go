// Package normalizer provides functionality for normalizing crawled data into standard formats.
package normalizer

import (
	"fmt"
)

// Processor handles data processing and transformation.
type Processor struct {
	validator   *Validator
	transformer *Transformer
}

// NewProcessor creates a new processor instance.
func NewProcessor() *Processor {
	return &Processor{
		validator:   NewValidator(),
		transformer: NewTransformer(),
	}
}

// Process transforms raw data into normalized format.
func (p *Processor) Process(rawData interface{}) (interface{}, error) {
	// 1. Validate the input data
	if err := p.validator.Validate(rawData); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 2. Transform the data
	normalizedData, err := p.transformer.Transform(rawData)
	if err != nil {
		return nil, fmt.Errorf("transformation failed: %w", err)
	}

	return normalizedData, nil
}
