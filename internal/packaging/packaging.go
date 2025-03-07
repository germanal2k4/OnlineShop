package packaging

import (
	"fmt"
)

type PackagingType string

const (
	PackagingBag  PackagingType = "bag"
	PackagingBox  PackagingType = "box"
	PackagingFilm PackagingType = "film"
)

type Packaging interface {
	Validate(weight float64) error
	Cost() float64
	Type() PackagingType
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

func (p BagPackaging) Type() PackagingType {
	return PackagingBag
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

func (p BoxPackaging) Type() PackagingType {
	return PackagingBox
}

type FilmPackaging struct{}

func (p FilmPackaging) Validate(float64) error {
	return nil
}

func (p FilmPackaging) Cost() float64 {
	return 1.0
}

func (p FilmPackaging) Type() PackagingType {
	return PackagingFilm
}

type PackagingService interface {
	GetPackaging(pt PackagingType) (Packaging, error)
	ListPackaging() []PackagingType
}

type packagingService struct {
	types map[PackagingType]Packaging
}

func NewPackagingService() PackagingService {
	return &packagingService{
		types: map[PackagingType]Packaging{
			PackagingBag:  BagPackaging{},
			PackagingBox:  BoxPackaging{},
			PackagingFilm: FilmPackaging{},
		},
	}
}

func (ps *packagingService) GetPackaging(pt PackagingType) (Packaging, error) {
	if pkg, ok := ps.types[pt]; ok {
		return pkg, nil
	}
	return nil, fmt.Errorf("неподдерживаемый тип упаковки: %s", pt)
}

func (ps *packagingService) ListPackaging() []PackagingType {
	var list []PackagingType
	for k := range ps.types {
		list = append(list, k)
	}
	return list
}
