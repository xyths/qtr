package grid

import (
	"github.com/jinzhu/gorm"
	"github.com/xyths/hs"
)

type Order struct {
	gorm.Model
	Grid    int    // grid id
	OrderId uint64 // order id
}

type Base struct {
	gorm.Model
	Symbol string
	Grid   int // base grid id
}

type Param hs.Grid

type Grid struct {
	Param
	Order
}
