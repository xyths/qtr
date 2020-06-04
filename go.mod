module github.com/xyths/qtr

go 1.13

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/google/go-cmp v0.3.0 // indirect
	github.com/huobirdcenter/huobi_golang v0.0.0-00010101000000-000000000000
	github.com/jinzhu/gorm v1.9.12
	github.com/karalabe/xgo v0.0.0-20191115072854-c5ccff8648a7 // indirect
	github.com/klauspost/compress v1.10.3 // indirect
	github.com/mattn/go-sqlite3 v2.0.1+incompatible
	github.com/nntaoli-project/goex v1.1.1
	github.com/pkg/errors v0.9.1
	github.com/shopspring/decimal v1.2.0
	github.com/stretchr/testify v1.4.0
	github.com/urfave/cli/v2 v2.2.0
	github.com/xyths/hs v0.0.0-00010101000000-000000000000
	go.mongodb.org/mongo-driver v1.3.2
	golang.org/x/crypto v0.0.0-20200403201458-baeed622b8d8 // indirect
	golang.org/x/net v0.0.0-20190827160401-ba9fcec4b297
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a // indirect
)

replace github.com/huobirdcenter/huobi_golang => ../../huobirdcenter/huobi_Golang

replace github.com/xyths/hs => ../hs
