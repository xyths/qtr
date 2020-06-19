# qtr
Quantitative trading robot

## 网格交易

**`print`** 打印网格

```shell
./bin/qtr -c config/config.json grid print
```

**`rebalance`** 自动平衡

```shell
./bin/qtr -c config/config.json grid rebalance --dry-run
```

**`grid`** 运行网格

```shell
./bin/qtr -c config/config.json grid
```

**`clear`** 清空网格并取消订单

```shell
./bin/qtr -c config/config.json grid clear
```