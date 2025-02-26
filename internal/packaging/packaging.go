package packaging

import (
	"fmt"
	"strings"
)

type Packaging interface {
	Validate(weight float64) error
	Cost() float64
}

type BagPackaging struct{}

func (p BagPackaging) Validate(weight float64) error {
	if weight >= 10 {
		return fmt.Errorf("для упаковки 'пакет' вес заказа должен быть меньше 10 кг, а вес = %.2f", weight)
	}
	return nil
}

func (p BagPackaging) Cost() float64 {
	return 5.0
}

type BoxPackaging struct{}

func (p BoxPackaging) Validate(weight float64) error {
	if weight >= 30 {
		return fmt.Errorf("для упаковки 'коробка' вес заказа должен быть меньше 30 кг, а вес = %.2f", weight)
	}
	return nil
}

func (p BoxPackaging) Cost() float64 {
	return 20.0
}

type FilmPackaging struct{}

func (p FilmPackaging) Validate(weight float64) error {
	return nil
}

func (p FilmPackaging) Cost() float64 {
	return 1.0
}

func NewPackaging(pack string) (Packaging, error) {
	switch strings.ToLower(strings.TrimSpace(pack)) {
	case "пакет", "package":
		return BagPackaging{}, nil
	case "коробка", "box":
		return BoxPackaging{}, nil
	case "пленка", "film":
		return FilmPackaging{}, nil
	default:
		return nil, fmt.Errorf("неизвестный тип упаковки: %s", pack)
	}
}
